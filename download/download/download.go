package download

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"path/filepath"
	"strconv"
	"download/x/logger"
)

type Download struct {
	content_chan chan map[int64]string
	file_chan    chan string
	Url string
	Method string
	GSize int64
	log logger.Logger
}

func NewDownload(size int64) *Download {
	return &Download{
		content_chan: make(chan map[int64]string, size),
		file_chan: make(chan string, 1),
		GSize:size,
		log:logger.NewStdLogger(true, true, true, true, true),
	}
}

func (d *Download) DownloadByGet() error{
	var (
		idx   int64 = 0
		unit int64 = 0
		cl int64
		name string
		ac bool
		err error
	)
	name = d.Url[strings.LastIndex(d.Url, "/")+1:]
	ac, cl, err = d.getContentLength(d.Url)
	if err != nil{
		return err
	}
	if !ac{
		d.GSize = 1
	}
	unit = cl/d.GSize
	go d.converToFile(name)
	go d.converToOne()
	for {
		go func(idx int64) {
			start := idx * unit
			end := (idx+1)*unit
			if idx == d.GSize{
				end = 0 // 0 表示读到最后
			}
			d.download(d.Url, d.Method, start, end, idx)
		}(idx)
		idx++
		if idx >= d.GSize {
			break
		}
	}
	return nil
}

func (d *Download) getContentLength(url string)(acceptRanges bool, contentL int64, err error){
	var(
		req *http.Request
		rsp *http.Response
	)
	req, err = http.NewRequest("HEAD", url, nil)
	if err != nil {
		d.log.Error("创建连接失败：%s", err)
		return
	}
	rsp, err = http.DefaultClient.Do(req)
	if err != nil {
		d.log.Error("调用连接失败：%s", err)
		return
	}
	cl := rsp.Header.Get("Content-Length")
	acceptRanges = func()bool{
		ar := rsp.Header.Get("Accept-Ranges")
		if ar == "bytes"{
			return true
		}
		return false
	}()
	contentL, err = strconv.ParseInt(cl, 10, 64)
	if err != nil {
		d.log.Error("转换请求响应长度失败：%s", err)
		return
	}
	return
}

func (d *Download) download(url, method string, start, end, idx int64) error{
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		d.log.Error("创建连接失败：%s", err)
		return err
	}

	rang := fmt.Sprintf("bytes=%d-%d", start, end)
	if end == 0 {
		rang = fmt.Sprintf("bytes=%d-", start)
	}
	req.Header.Set("Range", rang)
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		d.log.Error("调用连接失败：%s", err)
		return err
	}
	content, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		d.log.Error("读取返回值失败：%s", err)
		return err
	}
	if !strings.Contains(string(content), "416 Requested Range Not Satisfiable") {
		d.content_chan <- map[int64]string{idx: string(content)}
	}
	return nil
}

func (d *Download) converToOne() {
	var (
		contents []string = make([]string, d.GSize, d.GSize)
		count int64
	)
	for {
		c := <-d.content_chan
		for k, v := range c {
			contents[k] = v
		}
		
		if count < d.GSize{
			break
		}
		count ++
	}
	d.file_chan <- strings.Join(contents, "")
}

func (d *Download) converToFile(name string) error{
	c := <-d.file_chan
	fmt.Println("总共下了多少：", len(c))
	dir, err := filepath.Abs("./")
	if err != nil {
		d.log.Error("获取当前文件路径失败：%s", err)
		return err
	}
	file, err := os.Create(fmt.Sprintf("%s\\src\\file\\%s", dir, name))
	if err != nil {
		d.log.Error("创建文件失败：%s", err)
		return err
	}
	_, err = file.WriteString(c)
	if err != nil {
		d.log.Error("写入文件失败：%s", err)
		return err
	}
	file.Close()
	return nil
}
