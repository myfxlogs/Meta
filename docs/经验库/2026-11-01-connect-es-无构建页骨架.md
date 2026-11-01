# connect-es 无构建页骨架（票0012）

坑：浏览器无构建步用 connect-es，`@bufbuild/protobuf` 根包不导出
`fileDesc`，且 `createFileRegistry` 不吃 `DescFile`——两 API 都在
v2 换过面。报现象：import 无 `fileDesc` 名 / `fileReg.files is not iterable`。

解：

- `fileDesc` 在 `@bufbuild/protobuf/codegenv2` 子路径；签名
  `fileDesc(b64, imports? DescFile[]) → DescFile`——**它内部已建
  registry**，返回的 DescFile 直接取 `.services`/`.messages` 数组
  `find(typeName)`，不需要再过 `createFileRegistry`。
- 描述符来源：`protoc --include_imports -o x.desc`，把集合里每个
  `FileDescriptorProto` 各自 marshal→base64 内嵌；deps 传 DescFile 对象
  （不是 b64）。
- CDN 多包要钉同一 protobuf 版本——esm.sh `?deps=@bufbuild/protobuf@X`
  把内部 import 改指同一 URL，防双实例。
- connect-es 客户端：`createClient(DescService, createConnectTransport({baseUrl}))`
  → `for await (const m of client.method({...},{signal}))`。
- 服务端 CORS：connect-web 的 `application/connect+proto` 是非简单
  Content-Type，必过 OPTIONS 预检——Allow-Headers 要含
  `Content-Type, Connect-Protocol-Version`。
- connect-go v2 API 面：`connect.NewServer()`＋`pkgconnect.RegisterXxxHandler(srv)`
  ＋`connecthttp.Mount(mux, srv)`；`connect.NewError(code, string)`
  收字符串不是 error；客户端 `connecthttp.NewTransport(httpClient, baseURL)`
  →`connect.NewClient(tr)`。
