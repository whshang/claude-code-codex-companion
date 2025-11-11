package starlark

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Executor 负责执行Starlark脚本
type Executor struct {
	script  string
	name    string
	timeout time.Duration
}

// NewExecutor 创建新的Starlark执行器
func NewExecutor(name, script string, timeout time.Duration) *Executor {
	return &Executor{
		script:  script,
		name:    name,
		timeout: timeout,
	}
}

// ExecuteScript 执行Starlark脚本判断是否应该添加tag
func (e *Executor) ExecuteScript(req *http.Request) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	// 创建Starlark执行环境
	thread := &starlark.Thread{Name: e.name}
	
	// 设置超时控制
	done := make(chan error, 1)
	var result bool
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("starlark script panic: %v", r)
			}
		}()
		
		// 创建内置函数和变量
		predeclared := e.createPredeclaredEnvironment(req)
		
		// 执行脚本
		globals, err := starlark.ExecFile(thread, e.name+".star", e.script, predeclared)
		if err != nil {
			done <- fmt.Errorf("starlark execution error: %v", err)
			return
		}
		
		// 调用should_tag函数
		shouldTagFunc, exists := globals["should_tag"]
		if !exists {
			done <- fmt.Errorf("should_tag function not found in script")
			return
		}
		
		function, ok := shouldTagFunc.(*starlark.Function)
		if !ok {
			done <- fmt.Errorf("should_tag is not a function")
			return
		}
		
		// 调用函数
		resultValue, err := starlark.Call(thread, function, nil, nil)
		if err != nil {
			done <- fmt.Errorf("error calling should_tag: %v", err)
			return
		}
		
		// 转换结果
		if boolResult, ok := resultValue.(starlark.Bool); ok {
			result = bool(boolResult)
			done <- nil
		} else {
			done <- fmt.Errorf("should_tag must return a boolean, got %T", resultValue)
		}
	}()
	
	select {
	case err := <-done:
		return result, err
	case <-ctx.Done():
		return false, fmt.Errorf("starlark script execution timeout")
	}
}

// createPredeclaredEnvironment 创建Starlark脚本的预定义环境
func (e *Executor) createPredeclaredEnvironment(req *http.Request) starlark.StringDict {
	// 创建请求对象
	requestObj := e.createRequestObject(req)
	
	// 创建内置函数
	predeclared := starlark.StringDict{
		"request": requestObj,
		"len":     starlark.NewBuiltin("len", starlarkLen),
		"str":     starlark.NewBuiltin("str", starlarkStr),
		"lower":   starlark.NewBuiltin("lower", starlarkLower),
		"upper":   starlark.NewBuiltin("upper", starlarkUpper),
		"contains": starlark.NewBuiltin("contains", starlarkContains),
		"startswith": starlark.NewBuiltin("startswith", starlarkStartswith),
		"endswith": starlark.NewBuiltin("endswith", starlarkEndswith),
	}
	
	// 添加struct模块用于创建对象
	structModule := &starlarkstruct.Module{
		Name: "struct",
		Members: starlark.StringDict{
			"struct": starlark.NewBuiltin("struct", starlarkstruct.Make),
		},
	}
	predeclared["struct"] = structModule
	
	return predeclared
}

// createRequestObject 创建HTTP请求的Starlark对象
func (e *Executor) createRequestObject(req *http.Request) *starlarkstruct.Struct {
	// 创建headers字典
	headersDict := starlark.NewDict(len(req.Header))
	for key, values := range req.Header {
		// 将多个值用逗号连接
		value := strings.Join(values, ", ")
		headersDict.SetKey(starlark.String(key), starlark.String(value))
	}
	
	// 创建query parameters字典
	queryDict := starlark.NewDict(len(req.URL.Query()))
	for key, values := range req.URL.Query() {
		value := strings.Join(values, ", ")
		queryDict.SetKey(starlark.String(key), starlark.String(value))
	}
	
	// 创建请求对象
	requestData := starlark.StringDict{
		"method":  starlark.String(req.Method),
		"path":    starlark.String(req.URL.Path),
		"query":   starlark.String(req.URL.RawQuery),
		"headers": headersDict,
		"params":  queryDict,
		"host":    starlark.String(req.Host),
		"scheme":  starlark.String(req.URL.Scheme),
	}
	
	return starlarkstruct.FromStringDict(starlarkstruct.Default, requestData)
}

// Starlark内置函数实现

func starlarkLen(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var x starlark.Value
	if err := starlark.UnpackArgs("len", args, kwargs, "x", &x); err != nil {
		return nil, err
	}
	
	switch v := x.(type) {
	case starlark.String:
		return starlark.MakeInt(len(string(v))), nil
	case *starlark.List:
		return starlark.MakeInt(v.Len()), nil
	case *starlark.Dict:
		return starlark.MakeInt(v.Len()), nil
	default:
		return nil, fmt.Errorf("len() not supported for type %T", x)
	}
}

func starlarkStr(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var x starlark.Value
	if err := starlark.UnpackArgs("str", args, kwargs, "x", &x); err != nil {
		return nil, err
	}
	return starlark.String(x.String()), nil
}

func starlarkLower(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s starlark.String
	if err := starlark.UnpackArgs("lower", args, kwargs, "s", &s); err != nil {
		return nil, err
	}
	return starlark.String(strings.ToLower(string(s))), nil
}

func starlarkUpper(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s starlark.String
	if err := starlark.UnpackArgs("upper", args, kwargs, "s", &s); err != nil {
		return nil, err
	}
	return starlark.String(strings.ToUpper(string(s))), nil
}

func starlarkContains(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s, substr starlark.String
	if err := starlark.UnpackArgs("contains", args, kwargs, "s", &s, "substr", &substr); err != nil {
		return nil, err
	}
	result := strings.Contains(string(s), string(substr))
	return starlark.Bool(result), nil
}

func starlarkStartswith(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s, prefix starlark.String
	if err := starlark.UnpackArgs("startswith", args, kwargs, "s", &s, "prefix", &prefix); err != nil {
		return nil, err
	}
	result := strings.HasPrefix(string(s), string(prefix))
	return starlark.Bool(result), nil
}

func starlarkEndswith(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var s, suffix starlark.String
	if err := starlark.UnpackArgs("endswith", args, kwargs, "s", &s, "suffix", &suffix); err != nil {
		return nil, err
	}
	result := strings.HasSuffix(string(s), string(suffix))
	return starlark.Bool(result), nil
}