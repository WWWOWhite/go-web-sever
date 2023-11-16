package main

import (
	"fmt"
	"go-web-sever/framework"
)

func UserLoginController(c *framework.Context) error {
	fmt.Println("qingqiu")
	c.Json(200, "ok,UserLoginController")
	return nil
}
