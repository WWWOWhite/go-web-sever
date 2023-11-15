package main

import (
	"go-web-sever/framework"
	"net/http"
)

func main() {
	server := &http.Server{
		//	自定义请求核心处理函数
		Handler: framework.NewCore(),
		Addr:    ":8080",
	}
	server.ListenAndServe()
}
