# TokenKey 补丁索引

> 本文件记录TT对New-API Fork的所有补丁，用于追踪差异化和上游同步。

## 补丁列表

| 编号 | 文件 | 类型 | 说明 | 回收条件 |
|------|------|------|------|---------|
| TT-001 | middleware/security_proxy.go | 新增 | 安全代理中间件，实现分级日志和不落盘承诺 | 上游实现类似功能 |
| TT-002 | middleware/admin_isolation.go | 新增 | 管理后台隔离中间件，路由隔离+审计+TOTP | 上游实现类似功能 |
| TT-003 | model/billing_enhanced.go | 新增 | 计费增强模型：注册赠送、邀请裂变、月套餐 | 上游实现类似功能 |

## 上游同步检查清单

每次上游同步后，检查以下内容：

- [ ] security_proxy.go 是否与上游middleware冲突
- [ ] admin_isolation.go 是否与上游middleware冲突
- [ ] billing_enhanced.go 是否与上游model冲突
- [ ] 运行回归测试：OpenAI协议、Anthropic协议、计费、管理端

## 上游同步流程

```bash
# 1. 拉取上游更新
git fetch upstream

# 2. 合并上游主分支
git merge upstream/main

# 3. 解决冲突（如有）
# 冲突通常集中在TT扩展层，不应影响核心主干

# 4. 运行回归测试
go test ./...

# 5. 推送到fork
git push origin main
```

## 版本历史

| 日期 | 上游版本 | TT版本 | 说明 |
|------|---------|--------|------|
| 2026-04-05 | 873067f | v0.1.0 | 初始Fork，添加TT核心中间件和计费模型 |
