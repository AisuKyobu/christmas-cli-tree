# 🎄 Command Line Christmas Tree Animator

这是一个基于 Go 语言和 tcell 库实现的命令行圣诞树动画程序。
## ✨ 特性

* **3D 螺旋轨迹**：星星围绕圣诞树进行 3D 螺旋运动。
* **背光透视**：星星绕到树后时，光线从树后透出，营造层次感。
* **粒子特效**：星星拖着彩色的粒子尾迹。
* **丰富装饰**：包含彩灯、挂饰和树下礼物。
* **动态居中**：根据终端大小自动调整树的位置。

## 🚀 如何运行

1.  **克隆仓库**
    \`\`\`bash
    git clone [您的仓库地址]
    cd christmas-cli-tree
    \`\`\`
2.  **运行程序**
    \`\`\`bash
    go run main.go
    \`\`\`
3.  **打包为 EXE (Windows)**
    \`\`\`bash
    # 在 PowerShell 中
    $env:GOOS=\"windows\"; $env:GOARCH=\"amd64\"; go build -o christmas-tree.exe
    # 在 CMD 中
    set GOOS=windows
    set GOARCH=amd64
    go build -o christmas-tree.exe
    \`\`\`
    运行生成的 \`christmas-tree.exe\` 文件即可。

---

## 🤖 AI 声明

**此项目的基础代码逻辑、结构设计及复杂算法（如 3D 螺旋计算、粒子系统）主要由 Google Gemini AI 协助编写和优化。**

该 $\text{AI}$ 提供了实现用户复杂动态效果的详细代码和技术指导，并帮助完成了 $\text{Git}$ 仓库的初始化和配置工作。
