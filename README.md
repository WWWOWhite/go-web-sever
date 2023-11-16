# go-web-sever



## 

![img](https://static001.geekbang.org/resource/image/3a/cd/3ab5c45e113ddf4cc3bdb0e09c85c7cd.png?wh=2464x1192)



## Context





## 路由

![img](https://static001.geekbang.org/resource/image/11/7b/11dee96201a6f32358d8cceced0f137b.jpg?wh=1920x1080)

框架设计者希望使用者如何用路由模块。

### 路由规则的需求

**需求 1：HTTP 方法匹配**

早期的 WebService 比较简单，HTTP 请求体中的 Request Line 或许只会使用到 Request-URI 部分，但是随着 REST 风格 WebService 的流行，为了让 URI 更具可读性，在现在的路由输入中，HTTP Method 也是很重要的一部分了，所以，我们框架也需要支持多种 HTTP Method，比如 GET、POST、PUT、DELETE。

**需求 2：静态路由匹配**

静态路由匹配是一个路由的基本功能，指的是路由规则中没有可变参数，即路由规则地址是固定的，与 Request-URI 完全匹配。我们在第一讲中提到的 DefaultServerMux 这个路由器，从内部的 map 中直接根据 key 寻找 value ，这种查找路由的方式就是静态路由匹配。

**需求 3：批量通用前缀**

因为业务模块的划分，我们会同时为某个业务模块注册一批路由，所以在路由注册过程中，为了路由的可读性，一般习惯统一定义这批路由的通用前缀。比如 /user/info、/user/login 都是以 /user 开头，很方便使用者了解页面所属模块。所以如果路由有能力统一定义批量的通用前缀，那么在注册路由的过程中，会带来很大的便利。

**需求 4：动态路由匹配**

这个需求是针对需求 2 改进的，因为 URL 中某个字段或者某些字段并不是固定的，是按照一定规则（比如是数字）变化的。那么，我们希望路由也能够支持这个规则，将这个动态变化的路由 URL 匹配出来。所以我们需要，使用自己定义的路由来补充，只支持静态匹配的 DefaultServerMux 默认路由。

![img](https://static001.geekbang.org/resource/image/dc/62/dc6e322c49be2334954d85b9883d0862.jpg?wh=1920x1080)

接下来我们按框架使用者使用路由的顺序分成四步来完善这个结构：**定义路由 map、注册路由、匹配路由、填充 ServeHTTP 方法**。



①路由定义map

```go
func NewCore() *Core {
	//	定义二级目录
	getRouter := map[string]ControllerHandler{}
	postRouter := map[string]ControllerHandler{}
	putRouter := map[string]ControllerHandler{}
	deleteRouter := map[string]ControllerHandler{}

	//	将二级map写入一级map
	router := map[string]map[string]ControllerHandler{}
	router["GET"] = getRouter
	router["POST"] = postRouter
	router["PUT"] = putRouter
	router["DELETE"] = deleteRouter

	return &Core{router: router}

}
```



②路由注册

```go
func (c *Core) Get(url string, handler ControllerHandler) {
	upperUrl := strings.ToUpper(url)
	c.router["GET"][upperUrl] = handler
}

func (c *Core) Post(url string, handler ControllerHandler) {
	upperUrl := strings.ToUpper(url)
	c.router["POST"][upperUrl] = handler
}

func (c *Core) Put(url string, handler ControllerHandler) {
	upperUrl := strings.ToUpper(url)
	c.router["PUT"][upperUrl] = handler
}

func (c *Core) Delete(url string, handler ControllerHandler) {
	upperUrl := strings.ToUpper(url)
	c.router["DELETE"][upperUrl] = handler
}

```

> 我们这里将 URL 全部转换为大写了，在后续匹配路由的时候，也要记得把匹配的 URL 进行大写转换，这样我们的路由就会是“大小写不敏感”的，对使用者的容错性就大大增加了。



③匹配路由

```go
func (c *Core) FindOutByRequest(request *http.Request) ControllerHandler {
	uri := request.URL.Path
	method := request.Method
	upperMethod := strings.ToUpper(method)
	upperUri := strings.ToUpper(uri)

	//查找第一层map
	if methodHandlers, ok := c.router[upperMethod]; ok {
		//	查找第二层
		if handler, ok := methodHandlers[upperUri]; ok {
			return handler
		}
	}
	return nil
}
```



④ 填充ServeHttp方法

```go
func (c *Core) ServeHttp(response http.ResponseWriter, request *http.Request) {
	//	TODO
	ctx := NewContext(request, response)

	router := c.FindOutByRequest(request)

	if router == nil {
		ctx.Json(404, "not found")
		return
	}

	if err := router(ctx); err != nil {
		ctx.Json(500, "inner error")
		return
	}
}

```



### 实现批量通用前缀

```go
// Group struct 实现了IGroup
type Group struct {
  core   *Core
  prefix string
}

// 初始化Group
func NewGroup(core *Core, prefix string) *Group {
  return &Group{
    core:   core,
    prefix: prefix,
  }
}

// 实现Get方法
func (g *Group) Get(uri string, handler ControllerHandler) {
  uri = g.prefix + uri
  g.core.Get(uri, handler)
}

....

// 从core中初始化这个Group
func (c *Core) Group(prefix string) IGroup {
  return NewGroup(c, prefix)
}

```

接口设计



### 动态路由匹配

现在分析清楚了，我们开始动手实现 trie 树。还是照旧先明确下可以分为几步：

1. 定义树和节点的数据结构
2. 编写函数：“增加路由规则”
3. 编写函数：“查找路由”
4. 将“增加路由规则”和“查找路由”添加到框架中

trie.go

```go
package framework

import (
	"errors"
	"strings"
)

type Tree struct {
	root *node
}

type node struct {
	// 代表这个节点是否可以成为最终的路由规则。该节点是否能成为一个独立的uri, 是否自身就是一个终极节点
	isLast bool
	// uri中的字符串，代表这个节点表示的路由中某个段的字符串
	segment string
	// 代表这个节点中包含的控制器，用于最终加载调用
	handler ControllerHandler
	// 代表这个节点下的子节点
	childs []*node
}

func NewTree() *Tree {
	root := newNode()
	return &Tree{root}
}

func newNode() *node {
	return &node{
		isLast:  false,
		segment: "",
		childs:  []*node{},
	}
}

// 判断一个segment是否是通用segment，即以：开头
func isWildSegment(segment string) bool {
	return strings.HasPrefix(segment, ":")
}

// 过滤下一层满足segment规则的子节点
// 通配符的作用？？？？
func (n *node) filterChildNodes(segement string) []*node {
	if len(n.childs) == 0 {
		return nil
	}

	//???
	//如果segement是通配符，则所有下一层子节点都满足需求
	if isWildSegment(segement) {
		return n.childs
	}

	nodes := make([]*node, 0, len(n.childs))
	for _, cnode := range n.childs {
		if isWildSegment(cnode.segment) {
			// 如果下一层子节点有通配符，则满足需求
			nodes = append(nodes, cnode)
		} else if cnode.segment == segement {
			nodes = append(nodes, cnode)
		}
	}

	return nodes
}

// 判断路由是否已经在节点的所有子节点树中存在了
func (n *node) matchNode(uri string) *node {
	segments := strings.SplitN(uri, "/", 2)
	segment := segments[0]
	if !isWildSegment(segment) {
		segment = strings.ToUpper(segment)
	}

	cnodes := n.filterChildNodes(segment)

	if cnodes == nil || len(cnodes) == 0 {
		return nil
	}

	if len(segments) == 1 {
		//如果segment已经是最后一个节点，判断这些cnode是否有isLast标志
		for _, tn := range cnodes {
			if tn.isLast {
				return tn
			}
		}
		//都不是最后一个节点
		return nil
	}

	for _, tn := range cnodes {
		tnMatch := tn.matchNode(segments[1])
		if tnMatch != nil {
			return tnMatch
		}
	}

	return nil
}

// 增加路由节点
func (tree *Tree) AddRouter(uri string, handler ControllerHandler) error {
	n := tree.root

	if n.matchNode(uri) != nil {
		return errors.New("route exist:" + uri)
	}

	segments := strings.Split(uri, "/")
	//	对每个segment
	for index, segment := range segments {
		if !isWildSegment(segment) {
			segment = strings.ToUpper(segment)
		}
		isLast := index == len(segments)-1

		var objNode *node
		childNodes := n.filterChildNodes(segment)

		if len(childNodes) > 0 {
			for _, cnode := range childNodes {
				if cnode.segment == segment {
					objNode = cnode
					break
				}
			}
		}

		if objNode == nil {
			cnode := newNode()
			if isLast {
				cnode.isLast = true
				cnode.handler = handler
			}
			n.childs = append(n.childs, cnode)
			objNode = cnode
		}
		n = objNode
	}

	return nil
}

// 匹配uri
func (tree *Tree) FindHandler(uri string) ControllerHandler {
	//	直接复用matchNode函数,uri是不带通配符的地址
	matchNode := tree.root.matchNode(uri)
	if matchNode == nil {
		return nil
	}
	return matchNode.handler
}

```

