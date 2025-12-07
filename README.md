# IP & DNS Records Dashboard

### 创建挂载路径
```
mkdir data
```
### 生成配置文件、API Key
```
docker run --rm -u $(id -u):$(id -g) -v $(pwd)/data:/data:rw ghcr.io/404700/idrd:latest --init
```
### 使用docker-compose.yml 启动容器
```
mv data/docker-compose.yml ./
docker compose up -d
```
