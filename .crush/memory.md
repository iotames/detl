用户用中文交流，母语是中文。
所有交互、文档注释、代码注释等都用中文。

重要规则：
- 提交 git 仓库前必须经过用户明确确认。用户没说"提交"或"commit"，绝对不要提交。
- 项目的.md文件，必须时刻和代码功能保持同步，特别是和用户使用相关的部分。
- **交互式确认**：正式回答问题或执行任务前，如有不清楚的必须先询问细节，了解意图和需求后再回答或执行。避免一次性输出太多却不符合预期。

easydb 依赖管理流程：
detl 依赖 github.com/iotames/easydb，本地 easydb/ 已被 .gitignore 排除。
修改 easydb 需跨两个仓库操作，必须按以下顺序执行：

阶段一：本地开发（go.mod 有 replace 指令，用本地 easydb/ 编译测试）
阶段二：发布 easydb → 提交 easydb → git tag vX.Y.Z → git push → git push --tags
阶段三：更新 detl → 删 replace → go.mod 改 vX.Y.Z → go mod tidy → go build/ test
阶段四：提交 detl（easydb/ 自动被 .gitignore 排除）

常见错误：
- git push --tags 不能忘，tag 不会随 git push 自动推送
- 必须先发 easydb tag，再改 detl 依赖，顺序不可逆
- 删 replace 后必须 go mod tidy
