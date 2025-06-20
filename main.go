package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof" // 导入pprof包用于性能分析，但不直接使用其API

	"github.com/shiyanhui/dht" // 第三方DHT库
)

// 定义文件结构，表示种子中的文件信息
type file struct {
	Path   []interface{} `json:"path"`   // 文件路径（可能是多级目录）
	Length int           `json:"length"` // 文件大小（字节）
}

// 定义种子信息结构
type bitTorrent struct {
	InfoHash string `json:"infohash"`         // 种子info_hash的十六进制表示
	Name     string `json:"name"`             // 种子名称
	Files    []file `json:"files,omitempty"`  // 包含的文件列表（可选字段）
	Length   int    `json:"length,omitempty"` // 单文件种子时表示文件大小（可选字段）
}

func main() {
	// 启动pprof性能分析服务器（监听6060端口）
	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	// 1. 初始化DHT Wire组件（底层网络通信）
	// 参数说明：65536(最大节点数), 1024(工作协程数), 256(最大数据包大小)
	w := dht.NewWire(65536, 1024, 256)

	// 2. 启动goroutine处理DHT响应
	go func() {
		// 循环读取DHT响应通道
		for resp := range w.Response() {
			// 解码种子元数据（metadata）
			metadata, err := dht.Decode(resp.MetadataInfo)
			if err != nil {
				continue // 解码失败则跳过
			}

			// 类型断言，将metadata转换为map
			info := metadata.(map[string]interface{})

			// 检查是否有name字段（种子名称）
			if _, ok := info["name"]; !ok {
				continue // 没有name字段则跳过
			}

			// 构造bitTorrent结构体
			bt := bitTorrent{
				InfoHash: hex.EncodeToString(resp.InfoHash), // 转换info_hash为十六进制
				Name:     info["name"].(string),             // 获取种子名称
			}

			// 处理多文件种子情况
			if v, ok := info["files"]; ok {
				files := v.([]interface{}) // 类型断言为文件列表
				bt.Files = make([]file, len(files))

				// 遍历所有文件
				for i, item := range files {
					f := item.(map[string]interface{}) // 单个文件信息
					bt.Files[i] = file{
						Path:   f["path"].([]interface{}), // 文件路径
						Length: f["length"].(int),         // 文件大小
					}
				}
			} else if _, ok := info["length"]; ok {
				// 处理单文件种子情况
				bt.Length = info["length"].(int)
			}

			// 将结构体编码为JSON格式输出
			data, err := json.Marshal(bt)
			if err == nil {
				fmt.Printf("%s\n\n", data) // 打印JSON格式的种子信息
			}
		}
	}()

	// 3. 启动DHT Wire组件
	go w.Run()

	// 4. 配置DHT爬虫
	config := dht.NewCrawlConfig()

	// 设置当发现peer时的回调函数
	config.OnAnnouncePeer = func(infoHash, ip string, port int) {
		// 向DHT网络请求该种子的元数据
		w.Request([]byte(infoHash), ip, port)
	}

	// 5. 创建DHT爬虫实例
	d := dht.New(config)

	// 6. 运行DHT爬虫（主线程阻塞在这里）
	d.Run()
}
