package ebs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeAPI struct { calls []string; fail string }
func (f *fakeAPI) call(name string) error { f.calls=append(f.calls,name); if f.fail==name{return errors.New("forced "+name)};return nil }
func (f *fakeAPI) DescribeVolume(context.Context,string)(Volume,error){if err:=f.call("describe");err!=nil{return Volume{},err};return Volume{ID:"vol-source",SizeGiB:100,AvailabilityZone:"ap-northeast-1a",Type:"gp3",Encrypted:true},nil}
func (f *fakeAPI) CreateVolume(context.Context,Volume,int64)(string,error){if err:=f.call("create");err!=nil{return "",err};return "vol-target",nil}
func (f *fakeAPI) AttachVolume(_ context.Context,volume,_,device string)error{return f.call("attach:"+volume+":"+device)}
func (f *fakeAPI) DetachVolume(_ context.Context,volume,_ string)error{return f.call("detach:"+volume)}
func (f *fakeAPI) WaitAvailable(_ context.Context,volume string)error{return f.call("available:"+volume)}
func (f *fakeAPI) WaitInUse(_ context.Context,volume string)error{return f.call("in-use:"+volume)}
func (f *fakeAPI) DeleteVolume(_ context.Context,volume string)error{return f.call("delete:"+volume)}

type fakeMigrator struct{calls []string;fail string}
func(f *fakeMigrator)step(name string)error{f.calls=append(f.calls,name);if f.fail==name{return errors.New("forced "+name)};return nil}
func(f *fakeMigrator)Preflight(context.Context,ReplacementSpec,Volume)error{return f.step("preflight")}
func(f *fakeMigrator)Migrate(context.Context,ReplacementSpec,string)error{return f.step("migrate")}
func(f *fakeMigrator)Verify(context.Context,ReplacementSpec,string)error{return f.step("verify")}
func(f *fakeMigrator)Activate(context.Context,ReplacementSpec,string)error{return f.step("activate")}

type fakeJournal struct{saved []Operation;deleted []string;failPhase Phase}
func(f *fakeJournal)Save(_ context.Context,op Operation)error{f.saved=append(f.saved,op);if op.Phase==f.failPhase{return errors.New("journal failed")};return nil}
func(f *fakeJournal)Delete(_ context.Context,id string)error{f.deleted=append(f.deleted,id);return nil}
func spec()ReplacementSpec{return ReplacementSpec{OperationID:"resize-1",InstanceID:"i-0123456789abcdef0",SourceVolumeID:"vol-source",SourceDevice:"/dev/sdf",StagingDevice:"/dev/sdg",TargetSizeGiB:60}}

func TestReplacementMigratesVerifiesAndPromotesWithoutDeletingSource(t *testing.T){
	api:=&fakeAPI{};mig:=&fakeMigrator{};journal:=&fakeJournal{};op,err:=New(api,mig,journal).Replace(context.Background(),spec());if err!=nil{t.Fatal(err)};if op.Phase!=PhaseComplete||op.TargetVolumeID!="vol-target"{t.Fatalf("op=%#v",op)}
	wantAPI:=[]string{"describe","create","attach:vol-target:/dev/sdg","in-use:vol-target","detach:vol-source","available:vol-source","detach:vol-target","available:vol-target","attach:vol-target:/dev/sdf","in-use:vol-target"};if strings.Join(api.calls,"|")!=strings.Join(wantAPI,"|"){t.Fatalf("api calls=%v",api.calls)};if strings.Join(mig.calls,"|")!="preflight|migrate|verify|activate"{t.Fatalf("migrator=%v",mig.calls)};if len(journal.deleted)!=1||journal.deleted[0]!="resize-1"{t.Fatalf("deleted=%v",journal.deleted)};for _,call:=range api.calls{if call=="delete:vol-source"{t.Fatal("source volume must never be auto-deleted")}}
}
func TestReplacementFailsBeforeMutationWhenPreflightOrTargetSizeUnsafe(t *testing.T){api:=&fakeAPI{};mig:=&fakeMigrator{fail:"preflight"};journal:=&fakeJournal{};if _,err:=New(api,mig,journal).Replace(context.Background(),spec());err==nil{t.Fatal("preflight failure accepted")};if strings.Contains(strings.Join(api.calls,"|"),"create"){t.Fatalf("mutated before preflight: %v",api.calls)};unsafe:=spec();unsafe.TargetSizeGiB=100;api=&fakeAPI{};mig=&fakeMigrator{};if _,err:=New(api,mig,journal).Replace(context.Background(),unsafe);!errors.Is(err,core.ErrInvalidArgument){t.Fatalf("err=%v",err)};if len(api.calls)!=1||api.calls[0]!="describe"{t.Fatalf("calls=%v",api.calls)}}
func TestVerificationFailureCleansOnlyReplacementVolume(t *testing.T){api:=&fakeAPI{};mig:=&fakeMigrator{fail:"verify"};journal:=&fakeJournal{};_,err:=New(api,mig,journal).Replace(context.Background(),spec());if err==nil{t.Fatal("expected error")};joined:=strings.Join(api.calls,"|");if !strings.Contains(joined,"delete:vol-target")||strings.Contains(joined,"detach:vol-source"){t.Fatalf("unsafe cleanup=%v",api.calls)};if len(journal.deleted)==0{t.Fatal("pre-detach journal not cleared")}}
func TestCleanupDetachFailureRequiresRecoveryAndKeepsJournal(t *testing.T){api:=&fakeAPI{fail:"detach:vol-target"};mig:=&fakeMigrator{fail:"verify"};journal:=&fakeJournal{};op,err:=New(api,mig,journal).Replace(context.Background(),spec());if !errors.Is(err,core.ErrRecoveryRequired)||op.Phase!=PhaseRecoveryRequired{t.Fatalf("op=%#v err=%v",op,err)};joined:=strings.Join(api.calls,"|");if strings.Contains(joined,"delete:vol-target"){t.Fatalf("target deleted after ambiguous detach: %v",api.calls)};if len(journal.deleted)!=0{t.Fatalf("journal deleted despite failed cleanup: %v",journal.deleted)};if len(journal.saved)==0||journal.saved[len(journal.saved)-1].Phase!=PhaseRecoveryRequired{t.Fatalf("journal=%#v",journal.saved)}}
func TestFailureAfterSourceDetachIsRecoveryRequiredAndNeverDeletesVolumes(t *testing.T){api:=&fakeAPI{fail:"detach:vol-target"};mig:=&fakeMigrator{};journal:=&fakeJournal{};op,err:=New(api,mig,journal).Replace(context.Background(),spec());if !errors.Is(err,core.ErrRecoveryRequired)||op.Phase!=PhaseRecoveryRequired{t.Fatalf("op=%#v err=%v",op,err)};joined:=strings.Join(api.calls,"|");if strings.Contains(joined,"delete:vol-target")||strings.Contains(joined,"delete:vol-source"){t.Fatalf("destructive rollback after source detach: %v",api.calls)};if len(journal.saved)==0||journal.saved[len(journal.saved)-1].Phase!=PhaseRecoveryRequired{t.Fatalf("journal=%#v",journal.saved)}}
func TestJournalFailureAfterSourceDetachAlsoRequiresRecovery(t *testing.T){api:=&fakeAPI{};mig:=&fakeMigrator{};journal:=&fakeJournal{failPhase:PhaseSourceDetached};op,err:=New(api,mig,journal).Replace(context.Background(),spec());if !errors.Is(err,core.ErrRecoveryRequired)||op.Phase!=PhaseRecoveryRequired{t.Fatalf("op=%#v err=%v",op,err)}}
func TestFileJournalPersistsPrivateAtomicState(t *testing.T){root:=filepath.Join(t.TempDir(),"ops");journal:=NewFileJournal(root);op:=Operation{Version:1,ID:"replace-1",Phase:PhaseVerified};if err:=journal.Save(context.Background(),op);err!=nil{t.Fatal(err)};info,err:=os.Stat(filepath.Join(root,"replace-1.json"));if err!=nil{t.Fatal(err)};if info.Mode().Perm()&0o077!=0{t.Fatalf("mode=%o",info.Mode().Perm())};if err:=journal.Delete(context.Background(),op.ID);err!=nil{t.Fatal(err)};if _,err:=os.Stat(filepath.Join(root,"replace-1.json"));!os.IsNotExist(err){t.Fatalf("journal remains: %v",err)}}
