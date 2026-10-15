# 能力规则 v1 契约

记录的media_type=image/video由外部传入。schema为：

```json
{
  "version": 1,
  "fields": {
    "prompt": {"type":"string","min_length":1},
    "size": {"type":"string","enum":["1:1"]},
    "n": {"type":"integer","minimum":1,"maximum":4,"default":1},
    "image_urls": {"type":"array","max_items":4,"items":{"type":"string","format":"http_url"}},
    "metadata": {"type":"object","properties":{"resolution":{"type":"string","enum":["1K"]}}}
  },
  "modes": [{"id":"default","when":[],"required":["prompt"],"forbidden":[]}],
  "rules": [],
  "image_specs": [],
  "batch": {"native_max_results":1,"allow_split":true,"semantically_linked":false}
}
```

这是语法示例，不代表任一供应商模型承诺。model由引擎固定必填string≤200，不允许fields重新定义。固定顶层字段以PRD为准，未声明的可选字段也不允许传入。

Field支持type(string/number/integer/boolean/object/array)、required、nullable、enum、minimum/maximum、min_length/max_length、min_items/max_items、default、properties、items、format(http_url或空)。每个对象递归拒绝未知属性，配置本身也拒绝未知字段。数组必须声明items。默认值必须满足该Field；对象默认值递归应用时不覆盖显式值。

Mode为{id,when:Condition[],required:string[],forbidden:string[]}。when全部成立才匹配，空数组是默认匹配；modes省略或为空时隐含default模式，显式声明模式时请求必须恰有一个匹配。Condition为{path,op,value?}，op限定present/absent/empty/not_empty/equals；path为点分字段路径，不能引用未知字段。empty包含不存在、null、空字符串、空数组和空对象；false与0不算空，not_empty要求存在且非空；present仅检查成员存在。required为“存在且符合Field”，非空要求由Field限制；forbidden表示不允许显式/默认出现。

Rule为{kind,paths:string[],when:Condition[]}；when省略/空则总是应用。固定kind：at_least_one（至少存在一项）、mutually_exclusive（最多存在一项）、require_together（要么都不存在，要么全部存在）、requires（首项存在时其余全部必需）。不能执行自由表达式或远程引用。

ImageSpec为{aspect_ratio,resolution,width,height}；若有配置，使用最终有效size/metadata.resolution查唯一映射；缺省值仅按schema声明补齐，未命中拒绝。没有配置时不推算像素。size若出现仅允许正整数比例；不把auto当比例。metadata.output_size不开放；layer_decomposition=true不开放。

Batch为{native_max_results,allow_split,semantically_linked}；native_max_results≥1。n.maximum是整项能力上限，不能靠拆分绕过；native只是上游单次数量。语义关联任务不自动拆成独立请求。

Go纯核心导出：Compile(mediaType string,schemaJSON []byte)(*Compiled,error)；(*Compiled).Validate(raw []byte)(*NormalizedRequest,[]FieldError)；MatchCandidate(normalized *NormalizedRequest,candidate *Compiled)CandidateMatch。

FieldError={path,code,message}；NormalizedRequest={values,mode,provided_fields,image_spec?,requested_count}；CandidateMatch={supported,needs_split,native_batch_size,errors}。解析用UseNumber保留数字，递归检查重复键、大小写、单对象和EOF，不能将字符串数字或小数当integer。

限额：request1MiB，metadata256KiB，schema512KiB，深度8，节点256，模式32，规则128。格式校验只解析http/https URL，不下载素材；下载安全与SSRF在F06沿既有机制处理。
