package download

import (
	"download/x/logger"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	TRY_TIMES = 5
)

var (
	tasks chan Download
)

type Download struct {
	base
	file_chan   chan string
	nickName    string
	to_file_dir string
	count       int32
	end         chan bool
	idx         int64
}

type base struct {
	Url      string
	Method   string
	FileName string
	unit     int64
	tryTimes int
	GSize    int64
	log      *logger.Logger
}

func NewDownload(size int64) *Download {
	tasks = make(chan Download, size)
	return &Download{
		file_chan:  make(chan string, size),
		end:        make(chan bool, 1),
		base:base{
			GSize: size,
			log:   logger.NewStdLogger(true, true, true, true, true),
		},
	}
}

func (d *Download) successCount() {
	d.count = atomic.AddInt32(&d.count, 1)
	fmt.Println("完成了", d.count, "个")
	if int64(d.count) == d.GSize {
		d.end <- true
	}
}

func (d *Download) DownloadByGet() error {
	var (
		idx int64 = 0
		err error
	)

	d.nickName = fmt.Sprintf("%d", time.Now().Unix())
	d.count = 0
	if len(d.FileName) == 0 {
		d.FileName = d.Url[strings.LastIndex(d.Url, "/")+1:]
	}

	_, _, d.unit, err = d.getContentLength(d.Url)
	if err != nil {
		return err
	}

	dir, err := os.Getwd()
	if err != nil {
		d.log.Error("获取当前文件路径失败：%s", err)
		return err
	}

	d.to_file_dir = fmt.Sprintf("%s%sfile%s%s", dir, string(os.PathSeparator), string(os.PathSeparator), d.nickName)
	go d.writeToFile()
	go d.mergeFile()
	go d.download()

	for {
		d.idx = idx
		tasks <- *d
		d.log.Trace("任务放进去了吗？？？")
		idx++
		if idx >= d.GSize {
			break
		}
	}

	return nil
}

// 获取响应的总字节数，再根据线程多少，计算对应单元字节
func (d *Download) getContentLength(url string) (acceptRanges bool, contentL int64, unit int64, err error) {
	var (
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
	acceptRanges = func() bool {
		ar := rsp.Header.Get("Accept-Ranges")
		if ar == "bytes" {
			return true
		}
		return false
	}()
	contentL, err = strconv.ParseInt(cl, 10, 64)
	if err != nil {
		d.log.Error("转换请求响应长度失败：%s", err)
		return
	}
	if !acceptRanges {
		d.GSize = 1
	}
	unit = contentL / d.GSize
	d.log.Trace("是否支持range：%v, 单位：%d， 长度：%d, 线程数：%d", acceptRanges, unit, contentL, d.GSize)
	return
}

func (d *Download) download() error {
	for {
		select {
		case task := <-tasks:
			go task.exec()
		}
	}
	return nil
}

func (d *Download) exec() error{
	d.log.Trace("当前执行任务信息：%+v", d)
	if d.tryTimes > TRY_TIMES {
		d.log.Trace("超过重试次数。")
	}

	req, err := http.NewRequest(d.Method, d.Url, nil)
	if err != nil {
		d.tryTimes++
		tasks <- *d
		d.log.Error("创建连接失败：%s", err)
		return err
	}

	rang := fmt.Sprintf("bytes=%d-%d", d.idx*d.unit, (d.idx+1)*d.unit)
	if d.idx == d.GSize-1 {
		rang = fmt.Sprintf("bytes=%d-", d.idx*d.unit)
	}

	req.Header.Set("Range", rang)
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		d.tryTimes++
		tasks <- *d
		d.log.Error("调用连接失败：%s", err)
		return err
	}

	content, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		d.tryTimes++
		tasks <- *d
		d.log.Error("读取返回值失败：%s", err)
		return err
	}

	if !strings.Contains(string(content), "416 Requested Range Not Satisfiable") {
		d.file_chan <- fmt.Sprintf("%d_%s", d.idx, string(content))
	}
	return nil
}

func (d *Download) writeToFile() error {
	for {
		select {
		case c := <-d.file_chan:
			go d.write(c)
		}
	}
	return nil
}

func (d *Download) write(c string) error {
	idx := strings.Index(c, "_")
	content := c[idx+1:]
	idx64, _ := strconv.ParseInt(c[:idx], 10, 64)
	os.MkdirAll(d.to_file_dir, os.ModeDir)

	file, err := os.Create(fmt.Sprintf("%s%s%s_%d", d.to_file_dir, string(os.PathSeparator), d.nickName, idx64))
	if err != nil {
		d.log.Error("创建文件失败：%s", err)
		return err
	}
	_, err = file.WriteString(content)
	if err != nil {
		d.log.Error("写入文件失败：%s", err)
		return err
	}
	file.Close()
	go d.successCount()
	return nil
}

// 下载完成后，将所有文件合并成一个文件
func (d *Download) mergeFile() error {
	<-d.end
	files, err := ioutil.ReadDir(d.to_file_dir)
	if err != nil {
		d.log.Trace("下载的文件有异常，请重新下载。%s", err)
		return err
	}
	dest, err := os.Create(fmt.Sprintf("%s%s", d.to_file_dir[:strings.LastIndex(d.to_file_dir, string(os.PathSeparator))+1], d.FileName))
	if err != nil {
		d.log.Trace("生成最终目标文件失败。%s", err)
		return err
	}
	defer dest.Close()
	for _, f := range files {
		n := strings.LastIndex(f.Name(), "_")
		idx, err := strconv.ParseInt(f.Name()[n+1:], 10, 64)
		if err != nil {
			d.log.Trace("文件序列错误。%s", err)
		}
		path := fmt.Sprintf("%s%s%s", d.to_file_dir, string(os.PathSeparator), f.Name())
		content, err := ioutil.ReadFile(path)
		if err != nil {
			d.log.Trace("读取文件错误。%s", err)
		}
		dest.WriteAt(content, idx*d.unit)
	}
	go d.removeFile()
	d.log.Trace("下载完成。")
	return nil
}

func (d *Download) removeFile() {
	os.RemoveAll(d.to_file_dir)
}
