package tools

import (
 "context"
 "encoding/json"
 "errors"
 "strings"
 "testing"
 "bots/internal/provider"
)
type fixtureTool struct{executed *int}
func(f fixtureTool)Definition()provider.ToolDefinition{return provider.ToolDefinition{Name:"fixture"}}
func(f fixtureTool)Validate(raw json.RawMessage)error{if string(raw)!=`{"valid":true}`{return errors.New("invalid fixture arguments")};return nil}
func(f fixtureTool)Execute(context.Context,json.RawMessage)(string,error){*f.executed++;return strings.Repeat("x",100),nil}
func TestPermissionChecksPrecedeExecution(t *testing.T){
 count:=0;r:=New();r.OutputLimit=10;if e:=r.Register(fixtureTool{&count});e!=nil{t.Fatal(e)}
 cases:=[]struct{name,args string;grants []string;code string}{{"unknown",`{}`,nil,"unknown_tool"},{"fixture",`{"valid":true}`,nil,"permission_denied"},{"fixture",`{}`,[]string{"fixture"},"invalid_arguments"}}
 for _,c:=range cases{result:=r.Call(context.Background(),c.grants,provider.Call{Function:provider.Function{Name:c.name,Arguments:c.args}});if result.Error==nil||result.Error.Code!=c.code{t.Fatal(result,c.code)}}
 if count!=0{t.Fatal("denied call executed")};result:=r.Call(context.Background(),[]string{"fixture"},provider.Call{Function:provider.Function{Name:"fixture",Arguments:`{"valid":true}`}});if count!=1||len(result.Output)!=10||!result.Truncated{t.Fatal(count,result)}
}
