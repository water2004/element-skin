# --- 第一阶段：编译前端 ---
FROM node:20-alpine AS frontend-builder
WORKDIR /web
# 复制前端源码 (上下文为根目录)
COPY element-skin/package*.json ./
RUN npm install
COPY element-skin/ .
# 默认编译参数
ARG VITE_BASE_PATH=/
ARG VITE_API_BASE=
ENV VITE_BASE_PATH=${VITE_BASE_PATH}
ENV VITE_API_BASE=${VITE_API_BASE}
RUN npm run build

# --- 第二阶段：编译支持 Free Threading 的 Python ---
FROM debian:bookworm-slim AS python-builder
ENV PYTHON_VERSION=3.14.0
ENV PYTHON_URL=https://www.python.org/ftp/python/${PYTHON_VERSION}/Python-${PYTHON_VERSION}.tar.xz
RUN apt-get update && apt-get install -y \
    build-essential libssl-dev zlib1g-dev libncurses5-dev libncursesw5-dev \
    libreadline-dev libsqlite3-dev libgdbm-dev libdb5.3-dev libbz2-dev \
    libexpat1-dev liblzma-dev tk-dev libffi-dev wget xz-utils pkg-config \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN wget -O python.tar.xz "${PYTHON_URL}" \
    && tar -xJf python.tar.xz \
    && cd Python-${PYTHON_VERSION} \
    && ./configure --enable-optimizations --disable-gil --prefix=/opt/python-3.14-t \
    && make -j$(nproc) \
    && make install
ENV PATH="/opt/python-3.14-t/bin:$PATH"
# 修正路径：从根目录复制后端依赖文件
COPY skin-backend/requirements.txt /src/requirements.txt
RUN pip3 install --no-cache-dir -r /src/requirements.txt

# --- 第三阶段：最终镜像 ---
FROM debian:bookworm-slim
WORKDIR /app

# 复制 Python 环境
COPY --from=python-builder /opt/python-3.14-t /opt/python-3.14-t
ENV PATH="/opt/python-3.14-t/bin:$PATH"

# 复制前端编译产物到镜像内部
COPY --from=frontend-builder /web/dist /app/frontend_dist

# 安装运行时基础依赖
RUN apt-get update && apt-get install -y \
    libsqlite3-0 libssl3 ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 复制后端项目文件 (修正路径：仅复制后端目录内容)
COPY skin-backend/ .
RUN chmod +x entrypoint.sh

EXPOSE 8000
ENTRYPOINT ["./entrypoint.sh"]
CMD ["python3", "-m", "uvicorn", "routes_reference:app", "--host", "0.0.0.0", "--port", "8000", "--loop", "asyncio"]
