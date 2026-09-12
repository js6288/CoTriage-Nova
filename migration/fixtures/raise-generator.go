package raise
import("testing";"encoding/json";"fmt")
func TestNovaGolden(t *testing.T){
 c:=DefaultConfig(); e:=New(c); in:=baseEvent(); s:=healthyState();st:=highValueStats()
 cases:=[]Decision{e.Decide(in,s,st,Policy{})}
 in.EdgePred="NORMAL";in.Severity=5;in.Confidence=.99;in.Uncertainty=.01;in.InferencePath="RULE"
 cases=append(cases,e.Decide(in,s,Stats{TemplateFrequency:100,ConformalSetSize:1},Policy{}))
 in=baseEvent();in.Severity=99;in.Confidence=.2;in.Uncertainty=.8;in.InferencePath="FALLBACK";s.CloudHealth="DOWN";st.TemplateFrequency=0;st.HistoricalCorrectionCount=10;st.AffectedDevices=20;st.ConformalSetSize=8;st.UCBUncertainty=1
 cases=append(cases,e.Decide(in,s,st,Policy{})); b,_:=json.Marshal(cases);fmt.Println("NOVA_GOLDEN="+string(b))
}
