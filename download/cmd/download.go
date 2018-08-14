package main

import (
	"fmt"
	"net/http"
	"download/download"
)

type server struct {
	d *download.Download
}

func main() {
	defer func(){
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	s := &server{
		d: download.NewDownload(100),
	}
	http.HandleFunc("/download", s.dl)
	http.ListenAndServe("127.0.0.1:8080", nil)
}

func (s *server) dl(w http.ResponseWriter, r *http.Request){
	r.ParseForm()
	url := r.FormValue("url")
	method := r.FormValue("method")
	s.d.Url = url
	s.d.Method = method
	s.d.DownloadByGet()
	w.Write([]byte("下载成功！"))
}
