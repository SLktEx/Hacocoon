package awscap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type captureCapabilities struct{ req core.CapabilityRequest }
func(c *captureCapabilities)Request(_ context.Context,req core.CapabilityRequest)(core.CapabilityResult,error){c.req=req;return core.CapabilityResult{Provider:Capability},nil}
func TestBrokerMakesAWSAuthorityFullyPolicyVisible(t *testing.T){capture:=&captureCapabilities{};broker:=NewBroker(capture);_,err:=broker.Query(context.Background(),QuerySpec{Region:"ap-northeast-1",Kind:QueryInstance,ID:"i-0123456789abcdef0"});if err!=nil{t.Fatal(err)};req:=capture.req;if req.Capability!=Capability||req.Action!=ActionDescribeInstance||req.Resource!="aws://ap-northeast-1/ec2/instance/i-0123456789abcdef0"{t.Fatalf("req=%#v",req)};if len(req.Attributes)!=0||len(req.Parameters)!=0{t.Fatalf("attrs=%#v params=%#v",req.Attributes,req.Parameters)}}
func TestBrokerRejectsImplicitOrMalformedAuthority(t *testing.T){broker:=NewBroker(&captureCapabilities{});for _,spec:=range []QuerySpec{{Kind:QueryInstance,Region:"",ID:"i-0123456789abcdef0"},{Kind:QueryInstance,Region:"ap-northeast-1",ID:"../../bad"},{Kind:"mutate",Region:"ap-northeast-1"}}{if _,err:=broker.Query(context.Background(),spec);err==nil{t.Fatalf("accepted %#v",spec)}}}
type runner struct{call string;result host.Result;err error}
func(r *runner)Run(_ context.Context,name string,args ...string)(host.Result,error){r.call=name+" "+strings.Join(args," ");return r.result,r.err}
func TestProviderExecutesOnlyNormalizedReadOperations(t *testing.T){r:=&runner{result:host.Result{Stdout:`{"Reservations":[]}`}};p:=NewProvider(r);req,_:=requestFor(QuerySpec{Region:"ap-northeast-1",Kind:QueryInstance,ID:"i-0123456789abcdef0"});result,err:=p.Execute(context.Background(),req);if err!=nil||result.Output==""{t.Fatalf("result=%#v err=%v",result,err)};if !strings.Contains(r.call,"aws --region ap-northeast-1 --no-cli-pager ec2 describe-instances --instance-ids i-0123456789abcdef0 --output json"){t.Fatalf("call=%s",r.call)};for _,mutate:=range []func(*core.CapabilityRequest){func(r *core.CapabilityRequest){r.Resource="aws://ap-northeast-1/ec2/volume/vol-0123456789abcdef0"},func(r *core.CapabilityRequest){r.Action=ActionDescribeVolume}}{stale:=req;mutate(&stale);if _,err:=p.Execute(context.Background(),stale);err==nil||!(errors.Is(err,core.ErrCapabilityStale)||errors.Is(err,core.ErrUnsupported)){t.Fatalf("mutated request err=%v",err)}}}
