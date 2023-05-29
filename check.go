package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (

	// 排除的状态码
	excludeCodeFlag string
	// 特定的状态码
	codeFlag string

	// 扫描的目录
	dirFlag string
	// 排除的目录
	excludeDirFlag string

	// 排除的文件类型
	excludeFileFlag string
	// (要检测的)特定的文件类型
	fileFlag string

	// 同时请求的协程数
	concurrencyFlag int

	// 排除的域名中的关键字
	excludeKeywords string
	// 特定的域名中的关键字
	keywords string

	// 是否去除其他信息，仅保留有问题的链接
	pure bool
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] [DIR]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Scans for HTTP/HTTPS URLs in files under DIR, and checks their status codes.\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
}

func main() {

	go func() {
		fmt.Println(http.ListenAndServe(fmt.Sprintf("127.0.0.1:%s", "6060"), nil))
	}()

	//flag.StringVar(&excludeCodeFlag, "excludeCode,ec", "", "exclude HTTP status codes (comma-separated) | 要排除的状态码，英文逗号分隔多个")
	flag.StringVar(&codeFlag, "c", "", "specific HTTP status codes (comma-separated) | 要检测的状态码，英文逗号分隔多个")
	flag.StringVar(&excludeCodeFlag, "ec", "", "exclude HTTP status codes (comma-separated) | 要排除的状态码，英文逗号分隔多个")

	flag.StringVar(&dirFlag, "d", ".", "directory to scan | 要扫描的目录，默认为当前目录")
	flag.StringVar(&excludeDirFlag, "ed", "", "exclude directories (comma-separated) | 要排除的目录，英文逗号分隔多个")

	flag.StringVar(&fileFlag, "f", "", "specific file extensions (comma-separated) | 要检测的文件类型，英文逗号分隔多个")
	flag.StringVar(&excludeFileFlag, "ef", "", "exclude file extensions (comma-separated) | 要排除的文件类型，英文逗号分隔多个")

	flag.IntVar(&concurrencyFlag, "con", 10, "concurrency level | 并发协程数，默认为10")

	flag.StringVar(&keywords, "k", "", "specific keyword in url (comma-separated) | 要检测的url中的关键字，英文逗号分隔多个")
	flag.StringVar(&excludeKeywords, "ek", "", "exclude keyword in url (comma-separated) | 要排除的url中的关键字，英文逗号分隔多个")

	flag.BoolVar(&pure, "p", false, "pure output |  是否去除其他信息，仅保留有问题的链接")

	flag.Usage = usage
	flag.Parse()

	fmt.Println("用户输入的参数数量:", flag.NFlag())

	fmt.Println("pure is:", pure)

	// 输出帮助信息
	if flag.NFlag() == 0 {
		flag.Usage()
		return
	}

	ln, err := net.Listen("tcp", ":0") // 监听一个随机端口
	if err != nil {
		fmt.Println("error listening:", err)
		return
	}
	defer ln.Close()

	// 获取实际监听的地址和端口
	addr := ln.Addr().(*net.TCPAddr)
	fmt.Println("listening on", addr.String())
	fmt.Println("port:", addr.Port)

	initGlobal()

	fetchUrl(removeDuplEle(extractUrl()))

	time.Sleep(5e9)
	fmt.Println("最后！程序中goroutine的数量为:", runtime.NumGoroutine())

	ch := make(chan int, 0)

	ch <- 1

}

var (
	excludeCodeMap    = make(map[int]bool)
	codeMap           = make(map[int]bool)
	excludeDirMap     = make(map[string]bool)
	excludeFileMap    = make(map[string]bool)
	fileMap           = make(map[string]bool)
	excludeKeyWordMap = make(map[string]bool)
	keyWordMap        = make(map[string]bool)
)

func initGlobal() {

	// 将排除的状态码转化为一个 map
	for _, code := range strings.Split(excludeCodeFlag, ",") {
		if code != "" {
			codeInt, err := strconv.Atoi(code)
			if err != nil {
				panic("The entered status code format is incorrect | 输入的状态码格式有误")
			}

			excludeCodeMap[codeInt] = true
		}
	}

	if !pure {
		fmt.Println("要排除的状态码:", excludeCodeMap)
	}

	// 将指定的状态码转化为一个 map
	for _, code := range strings.Split(codeFlag, ",") {
		if code != "" {
			codeInt, err := strconv.Atoi(code)
			if err != nil {
				panic("The entered status code format is incorrect | 输入的状态码格式有误")
			}
			codeMap[codeInt] = true
		}
	}

	if len(codeMap) > 0 && len(excludeCodeMap) > 0 {
		panic("-c和-ec选项只能指定一个")
	}

	if !pure {
		fmt.Println("指定的状态码为:", codeMap)
	}

	// 将排除的目录转化为一个 map
	for _, dir := range strings.Split(excludeDirFlag, ",") {
		if dir != "" {
			excludeDirMap[dir] = true
		}
	}

	// 将排除的文件类型转化为一个 map
	for _, ext := range strings.Split(excludeFileFlag, ",") {
		if ext != "" {
			excludeFileMap[ext] = true
		}
	}

	if !pure {
		fmt.Println("要排除的文件类型:", excludeFileMap)

		fmt.Println("要扫描的路径为:", dirFlag)
	}

	// 将指定的文件类型转化为一个 map
	for _, ext := range strings.Split(fileFlag, ",") {
		if ext != "" {
			fileMap[ext] = true
		}
	}

	if len(fileFlag) > 0 && len(excludeFileFlag) > 0 {
		panic("-f和-ef选项只能指定一个")
	}

	// 将排除的关键字转化为一个 map
	for _, ext := range strings.Split(excludeKeywords, ",") {
		if ext != "" {
			excludeKeyWordMap[ext] = true
		}
	}

	// 将指定的关键字转化为一个 map
	for _, ext := range strings.Split(keywords, ",") {
		if ext != "" {
			keyWordMap[ext] = true
		}
	}

	if !pure {
		fmt.Println("要排除的关键字:", excludeKeyWordMap)
	}

}
func removeDuplEle(raw []string) []string {

	res := make([]string, 0)

	for _, val := range raw {

		if InSli(res, val) {
			continue
		} else {
			res = append(res, val)
		}

	}
	return res
}

func InSli(sli []string, ele string) bool {
	for _, val := range sli {
		if val == ele {
			return true
		}
	}
	return false
}

func extractUrl() []string {

	urlSli := make([]string, 0)
	if !pure {
		fmt.Println(1111)
		fmt.Println("fileType:", fileMap)
		fmt.Println("excludefileType:", excludeFileFlag)
	}
	// 遍历目录下的文件，查找匹配的 URL
	err := filepath.Walk(dirFlag, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// 检查是否需要排除该目录
			if excludeDirMap[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查是否需要排除该文件类型
		ext := filepath.Ext(path)
		if len(excludeFileMap) > 0 && excludeFileMap[ext] {
			return nil
		}

		// 检查是否是特定的文件类型
		if len(fileMap) > 0 && !fileMap[ext] {
			//	fmt.Println(2222)
			return nil
		}

		//	fmt.Println(33333)

		// 读取文件内容
		text, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		//fmt.Println("--------")
		//fmt.Println("路径为:", path)
		//	fmt.Println("文件内容为:",string(text))

		// 正则匹配出 URL
		re := regexp.MustCompile(`(?:https?://)[^\s]+`)

		urls := re.FindAllString(string(text), -1)

		//	fmt.Println("未处理的url数量为:", len(urls))

		for _, val := range urls {

			if !IsURL(val) {
				continue
			}

			// 处理 HTTP请求错误： Get "http://host/profile": EOF
			if !strings.Contains(val, ".") {
				continue
			}

			// 处理 http://www.w3.org/1999/xhtml";const
			// 处理 403: http://www.w3.org/2000/svg","g")),Yr.setAttribute("transform",t),(t=Yr.transform.baseVal.consolidate())?Gr((t=t.matrix).a,t.b,t.c,t.d,t.e,t.f):Zr)}),
			if strings.Contains(val, `"`) {
				val = strings.Split(val, `"`)[0]
			}

			// 处理 404: https://go.dev/blog/pprof'>pprof</a>'s
			if strings.Contains(val, `'`) {
				val = strings.Split(val, `'`)[0]
			}

			// 处理 https://go.dev/issue/new\n
			val = strings.TrimSuffix(val, "\n")

			val = strings.TrimSuffix(val, ".")

			val = strings.TrimSuffix(val, ",")
			val = strings.TrimSuffix(val, ")")
			val = strings.TrimSuffix(val, ")")
			val = strings.TrimSuffix(val, `"`)
			val = strings.TrimSuffix(val, `>`)
			val = strings.TrimSuffix(val, `]`)
			val = strings.TrimSuffix(val, `"`)
			val = strings.TrimSuffix(val, `:`)

			var containExcludeKey bool
			for i := range excludeKeyWordMap {
				if strings.Contains(val, i) {
					containExcludeKey = true
					break
				}
			}

			// 包含指定的要排除的关键字
			if len(excludeKeyWordMap) > 0 && containExcludeKey {
				continue
			}

			var containKeyWord bool
			for i := range keyWordMap {
				if strings.Contains(val, i) {
					containKeyWord = true
					break
				}
			}

			// 不包含指定的关键字
			if len(keyWordMap) > 0 && !containKeyWord {
				continue
			}

			if !pure {
				fmt.Println("扫描出且符合要求的url为:", val)
			}

			urlSli = append(urlSli, val)
		}

		return nil
	})

	if err != nil {
		fmt.Println("扫描出错,err:", err)
	}

	fmt.Println("总url数量为:", len(urlSli))
	return urlSli
}

func fetchUrl(urlSli []string) {

	for _, urlStr := range urlSli {
		worker2(urlStr)
	}
	return
	if !pure {
		fmt.Println("并发数量是:", concurrencyFlag)
	}

	// 创建一个有缓冲的channel，用于协程间通信
	ch := make(chan string, concurrencyFlag)

	// 启动10个协程
	for i := 0; i < concurrencyFlag; i++ {
		go worker(ch)
	}

	// 把链接放入channel中，让协程处理
	for _, urlStr := range urlSli {
		if len(urlStr) > 0 {
			ch <- urlStr
		}
	}

	// 关闭channel，等待所有协程执行完毕
	close(ch)
}

// 协程函数，用于处理链接
func worker(ch chan string) {
	for urlStr := range ch {

		if !pure {
			fmt.Println("当前程序中goroutine的数量为:", runtime.NumGoroutine())
		}
		// 创建一个自定义的 Transport 实例
		transport := &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				//	return url.Parse("http://127.0.0.1:1081")
				return url.Parse("socks5://127.0.0.1:1080")
			},
			MaxIdleConnsPerHost: 100,  // 每个主机最大空闲连接数
			MaxIdleConns:        1000, // 最大空闲连接数
		}

		client := &http.Client{
			Timeout:   time.Second * 300, // 设置超时时间为3秒
			Transport: transport,         // 设置代理
		}

		resp, err := client.Get(urlStr)
		if err != nil {

			if !pure {
				fmt.Println("HTTP请求错误：", err)
				fmt.Println("===============")
				fmt.Println()
			} else {
				//fmt.Println("HTTP请求错误：", err)
			}

			continue
		}
		defer resp.Body.Close()

		if !pure {
			fmt.Println("Response status:", resp.Status)
		}

		if len(codeMap) == 0 && len(excludeCodeMap) == 0 {
			// 检查HTTP响应状态码
			if resp.StatusCode == http.StatusOK {
				//fmt.Printf("链接%s返回值为200\n", url)
				//fmt.Println("===============")
				//fmt.Println()
			} else {
				if !pure {
					fmt.Printf("链接%s返回值不是200,而是%d\n", urlStr, resp.StatusCode)
					fmt.Println("===============")
					fmt.Println()
				} else {
					fmt.Printf("%d: %s\n", resp.StatusCode, urlStr)
				}

			}
		}

		for index := range codeMap {

			if resp.StatusCode == index {
				if !pure {
					fmt.Printf("链接%s返回的状态码为%d\n", urlStr, resp.StatusCode)
					fmt.Println("===============")
					fmt.Println()
				} else {
					fmt.Printf("%d: %s\n", resp.StatusCode, urlStr)
				}
			}
		}

		for index := range excludeCodeMap {
			if resp.StatusCode != index && resp.StatusCode != http.StatusOK {
				if !pure {
					fmt.Printf("链接%s返回的状态码为%d\n", urlStr, resp.StatusCode)
					fmt.Println("===============")
					fmt.Println()
				} else {
					fmt.Printf("%d: %s\n", resp.StatusCode, urlStr)
				}

			}
		}

	}
}

// 协程函数，用于处理链接
func worker2(urlStr string) {

	if !pure {
		fmt.Println("当前程序中goroutine的数量为:", runtime.NumGoroutine())
	}
	// 创建一个自定义的 Transport 实例
	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			//	return url.Parse("http://127.0.0.1:1081")
			return url.Parse("socks5://127.0.0.1:1080")
		},
		MaxIdleConnsPerHost: 100000,  // 每个主机最大空闲连接数
		MaxIdleConns:        1000000, // 最大空闲连接数
	}

	client := &http.Client{
		Timeout:   time.Second * 5, // 设置超时时间为3秒
		Transport: transport,       // 设置代理
	}

	resp, err := client.Get(urlStr)
	if err != nil {

		if !pure {
			fmt.Println("HTTP请求错误：", err)
			fmt.Println("===============")
			fmt.Println()
			return
		} else {
			fmt.Println("HTTP请求错误：", err)
			return
		}

		//continue
	}

	defer resp.Body.Close()

	if !pure && resp != nil {
		fmt.Println("Response status:", resp.Status)
	}

	if len(codeMap) == 0 && len(excludeCodeMap) == 0 {
		// 检查HTTP响应状态码
		if resp.StatusCode == http.StatusOK {
			//fmt.Printf("链接%s返回值为200\n", url)
			//fmt.Println("===============")
			//fmt.Println()
		} else {
			if !pure {
				fmt.Printf("链接%s返回值不是200,而是%d\n", urlStr, resp.StatusCode)
				fmt.Println("===============")
				fmt.Println()
			} else {
				fmt.Printf("%d: %s\n", resp.StatusCode, urlStr)
			}

		}
	}

	for index := range codeMap {

		if resp.StatusCode == index {
			if !pure {
				fmt.Printf("链接%s返回的状态码为%d\n", urlStr, resp.StatusCode)
				fmt.Println("===============")
				fmt.Println()
			} else {
				fmt.Printf("%d: %s\n", resp.StatusCode, urlStr)
			}
		}
	}

	for index := range excludeCodeMap {
		if resp.StatusCode != index && resp.StatusCode != http.StatusOK {
			if !pure {
				fmt.Printf("链接%s返回的状态码为%d\n", urlStr, resp.StatusCode)
				fmt.Println("===============")
				fmt.Println()
			} else {
				fmt.Printf("%d: %s\n", resp.StatusCode, urlStr)
			}

		}
	}

}
