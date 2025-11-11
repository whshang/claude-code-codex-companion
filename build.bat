@echo off
REM CCCC Proxy Desktop - Windows 构建脚本

setlocal enabledelayedexpansion

echo [INFO] 开始 CCCC Proxy Desktop Windows 构建...

REM 检查依赖
echo [INFO] 检查构建依赖...

where wails >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Wails CLI 未安装，请先安装: https://wails.io/docs/gettingstarted/installation/
    exit /b 1
)

where node >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Node.js 未安装，请先安装 Node.js
    exit /b 1
)

where go >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Go 未安装，请先安装 Go
    exit /b 1
)

echo [SUCCESS] 所有依赖检查通过

REM 清理构建目录
if "%1"=="--clean" (
    echo [INFO] 清理构建目录...
    if exist build\bin rmdir /s /q build\bin
    mkdir build\bin
    echo [SUCCESS] 构建目录已清理
)

REM 构建前端
echo [INFO] 构建前端应用...
cd frontend

REM 检查依赖
if not exist node_modules (
    echo [INFO] 安装前端依赖...
    npm install --legacy-peer-deps
)

REM 构建前端
npm run build

cd ..
echo [SUCCESS] 前端构建完成

REM 构建 Windows 应用
echo [INFO] 构建 Windows 应用...
wails build -platform windows/amd64 -clean

echo [SUCCESS] Windows 构建完成！

REM 列出生成的文件
if exist build\bin (
    echo [INFO] 生成的文件:
    dir /b build\bin\
)

echo [SUCCESS] 所有构建任务完成！
echo [INFO] 输出文件位置: build\bin\

pause