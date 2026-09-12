package infer

import (
    "encoding/json"
    "os"
    "testing"
)

// Reference-only characterization. Production source is not modified.
func TestMigrationGoldenCapture(t *testing.T) {
    if os.Getenv("EDGE_GOLDEN_OUTPUT") == "" { t.Skip("capture path not configured") }
    normalizeCases := []struct { Raw string `json:"raw"`; MaxBytes int `json:"max_bytes"` }{
        {"2026-06-22T03:00:00Z [pid=123] connect to 10.0.0.2:3306 at /var/lib/mysql/data",8192},
        {" UUID 550e8400-e29b-41d4-a716-446655440000 HEX 0xAb12 value 001 path /one/two.log ",8192},
        {" \t中文 😀 故障 123\u0085next\u00a0field\u001cend ",8192},
        {"中文😀suffix",1}, {"中文😀suffix",2}, {"中文😀suffix",4}, {"中文😀suffix",8},
        {"a\nb\r\nc\td",8192}, {"[thread=42] [pid=4] write failed /var/lib/data",8192},
        {"2026/06/22 03:00:00Z count=8 10.0.0.300",8192},
    }
    normalizations := []any{}
    for _, item := range normalizeCases { normalized:=Normalize(item.Raw,item.MaxBytes); normalizations=append(normalizations,map[string]any{"raw":item.Raw,"max_bytes":item.MaxBytes,"template_text":normalized,"template_id":TemplateID(normalized)}) }
    classifiers:=[]any{}
    for _, input:=range []ClassifierResult{{Label:"ALERT",LabelID:1,AlertProbability:.451605,Threshold:.25,ModelVersion:"bgl-distilbert"},{Label:"NON_ALERT",AlertProbability:.1,Threshold:.25},{Label:"ALERT",AlertProbability:.99,Threshold:.5}} {
        classifiers=append(classifiers,map[string]any{"input":input,"result":resultFromClassifier(input)})
    }
    llms:=[]any{}
    for _, content:=range []string{
        "```json\n{\"edge_pred\":\"NET_ERROR\",\"severity\":78,\"confidence\":0.72,\"evidence\":[\"connection refused\"],\"advice_code\":\"CHECK_NETWORK\",\"advice\":\"检查服务端口\"}\n```",
        `{"edge_pred":"NET_ERROR","severity":78,"confidence":0.72,"evidence":["connection refused"],"advice_code":"CHECK_NETWORK","advice":"检查`,
        `{"edge_pred":"ABNORMAL","severity":75,"confidence":0.7}`,
        `{"edge_pred":"SECURITY","severity":75,"confidence":0.7,"evidence":["1","2","3","4"]}`,
        `{"edge_pred":"APP_ERROR","severity":40,"confidence":0.6`,
        `no json output`,
        `{"edge_pred":"NET_ERROR","severity":78.5,"confidence":0.72}`,
    } { result,err:=parseLLMJSON(content); item:=map[string]any{"content":content,"error":err!=nil}; if err==nil { item["result"]=result }; llms=append(llms,item) }
    rules:=[]any{}; engine,_:=LoadRules("")
    for _, text:=range []string{"out of memory", "NO SPACE LEFT ON DEVICE", "connection refused", "authentication failed", "ordinary service log"} {
        result,matched:=engine.Match(text); rules=append(rules,map[string]any{"text":text,"matched":matched,"result":result})
    }
    content,err:=json.MarshalIndent(map[string]any{"source":"CoTriage-dev edge-infer Go 1.22.12","normalization":normalizations,"classifier":classifiers,"llm":llms,"rules":rules},"","  ")
    if err!=nil { t.Fatal(err) }; if err=os.WriteFile(os.Getenv("EDGE_GOLDEN_OUTPUT"),content,0600);err!=nil {t.Fatal(err)}
}
