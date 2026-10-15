# F03/F06 任务

- [x] T001 行为测试：状态投影、防重复提交、部分聚合。
- [x] T002 持久实体、加密快照、事务余额/Key预留。
- [x] T003 claim/fencing、账号共享额度及已受理绑定。
- [x] T004 worker提交/轮询/逐URL转存/有限失败恢复。
- [x] T005 终态聚合与幂等部分capture/release。
- [x] T006 真库并发、恢复、归属和Docker回归。

验证记录：最终串行 Docker 回归通过。真库覆盖并发 Key 预留、账号共享额度/并发、全部失败释放、部分结算幂等、归属隔离、租约接管、软删除凭据结算以及 n=1/n=3 的 HTTP 上游→worker→下载→存储元数据→正式扣费；新增逐结果转存和重启恢复单测同时通过。详见 [整体交付记录](../unified-media-generation/implementation-status.md)。
