# RMB Transcriber 部署文档

## 环境要求

- Docker 20.10+
- Docker Compose 1.29+（可选）
- 可用的8080端口

## 部署步骤

### 使用Docker部署

1. **克隆项目仓库**
   ```bash
   git clone https://github.com/12ain/rmb-transcriber.git
   cd rmb-transcriber
   ```

2. **构建Docker镜像**
   ```bash
   docker build -t rmb-transcriber .
   ```

3. **运行Docker容器**
   ```bash
   docker run -d -p 8080:8080 --name rmb-transcriber rmb-transcriber
   ```

4. **验证部署**
   打开浏览器访问 `http://localhost:8080`，如果看到应用界面，则部署成功。

### 使用Docker Compose部署

1. **创建docker-compose.yml文件**
   ```yaml
   version: '3'
   services:
     rmb-transcriber:
       build: .
       ports:
         - "8080:8080"
       restart: always
   ```

2. **启动服务**
   ```bash
   docker-compose up -d
   ```

3. **验证部署**
   同样访问 `http://localhost:8080` 验证应用是否正常运行。

## 配置说明

### 端口配置

默认情况下，应用使用8080端口。如果需要修改端口，可以：

1. 修改Docker运行命令中的端口映射：
   ```bash
   docker run -d -p 新端口:8080 --name rmb-transcriber rmb-transcriber
   ```

2. 或修改docker-compose.yml中的端口配置：
   ```yaml
   ports:
     - "新端口:8080"
   ```

### 生产环境配置

在生产环境中，建议：

1. 使用Nginx或Apache作为反向代理
2. 配置HTTPS证书
3. 设置适当的日志管理
4. 配置自动重启和监控

以下是一个简单的Nginx反向代理配置示例：
server {
    listen 80;
    server_name yourdomain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
## 更新应用

1. **拉取最新代码**
   ```bash
   cd rmb-transcriber
   git pull
   ```

2. **重建Docker镜像**
   ```bash
   docker build -t rmb-transcriber .
   ```

3. **重启容器**
   ```bash
   docker stop rmb-transcriber
   docker rm rmb-transcriber
   docker run -d -p 8080:8080 --name rmb-transcriber rmb-transcriber
   ```

## 故障排除

1. **检查容器状态**
   ```bash
   docker ps -a
   ```

2. **查看容器日志**
   ```bash
   docker logs rmb-transcriber
   ```

3. **进入容器内部**
   ```bash
   docker exec -it rmb-transcriber sh
   ```

如果遇到问题，可以检查端口占用情况、Docker网络配置或查看应用日志以获取更多信息。