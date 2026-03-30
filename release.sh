#!/bin/bash

# ==================== 自动发布脚本 ====================
# 功能：输入版本号 → 推送代码 → 打Tag → 执行 build.sh 编译 → 发布 GitHub Release
# ======================================================

# 1. 提示输入版本号
echo "====================================="
echo "        Go Web 版本发布工具"
echo "====================================="
read -p "请输入发布版本号（例如 v1.2）：" VERSION

# 2. 确认
echo -e "\n你将要发布的版本：\033[32m$VERSION\033[0m"
read -p "确定继续？(y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "发布已取消"
    exit 1
fi

# 3. 切换到 vue 分支
echo -e "\n\033[33m[1/6] 切换到 vue 分支...\033[0m"
git switch vue

# 4. 推送代码
echo -e "\033[33m[2/6] 推送 vue 分支代码...\033[0m"
git push origin vue

# 5. 打 tag
echo -e "\033[33m[3/6] 创建 tag：$VERSION\033[0m"
git tag -a "$VERSION" -m "发布 $VERSION"

# 6. 推送 tag
echo -e "\033[33m[4/6] 推送 tag 到远程...\033[0m"
git push origin "$VERSION"

# 7. 调用你的 build.sh 编译（重点在这里！）
echo -e "\033[33m[5/6] 执行 build.sh 编译程序...\033[0m"
chmod +x build.sh  # 确保可执行
./build.sh         # 运行你的编译脚本

# 8. 发布 GitHub Release
echo -e "\033[33m[6/6] 创建 GitHub Release 并上传 bin 文件...\033[0m"
gh release create "$VERSION" \
  --title "$VERSION 正式版" \
  --notes "发布 $VERSION 版本" \
  ./bin/*

# 完成
echo -e "\n\033[32m✅ 发布完成！版本：$VERSION\033[0m"
