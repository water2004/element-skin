# 第一阶段：编译支持 Free Threading 的 Python 3.14 并安装依赖
FROM debian:bookworm-slim AS builder

ENV PYTHON_VERSION=3.14.0
# 假设 2026 年 3 月 Python 3.14 已经发布正式版或候选版
ENV PYTHON_URL=https://www.python.org/ftp/python/${PYTHON_VERSION}/Python-${PYTHON_VERSION}.tar.xz

RUN apt-get update && apt-get install -y \
    build-essential \
    libssl-dev \
    zlib1g-dev \
    libncurses5-dev \
    libncursesw5-dev \
    libreadline-dev \
    libsqlite3-dev \
    libgdbm-dev \
    libdb5.3-dev \
    libbz2-dev \
    libexpat1-dev \
    liblzma-dev \
    tk-dev \
    libffi-dev \
    wget \
    xz-utils \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
RUN wget -O python.tar.xz "${PYTHON_URL}" \
    && tar -xJf python.tar.xz \
    && cd Python-${PYTHON_VERSION} \
    # 启用 free threading (--disable-gil)
    && ./configure --enable-optimizations --disable-gil --prefix=/opt/python-3.14-t \
    && make -j$(nproc) \
    && make install

# 在构建阶段安装 Python 依赖，以便编译 C 扩展（如 httptools, uvloop）
ENV PATH="/opt/python-3.14-t/bin:$PATH"
COPY requirements.txt /src/requirements.txt
RUN pip3 install --no-cache-dir -r /src/requirements.txt

# 第二阶段：运行阶段
FROM debian:bookworm-slim

WORKDIR /app

# 从构建阶段复制编译好的 Python（包含已安装的依赖）
COPY --from=builder /opt/python-3.14-t /opt/python-3.14-t
ENV PATH="/opt/python-3.14-t/bin:$PATH"

# 安装运行时的基础依赖（这些是动态链接库，必须在运行环境存在）
RUN apt-get update && apt-get install -y \
    libsqlite3-0 \
    libssl3 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 复制项目文件
COPY . .

# 赋予启动脚本执行权限
RUN chmod +x entrypoint.sh

EXPOSE 8000

ENTRYPOINT ["./entrypoint.sh"]
# 启动命令
CMD ["python3", "-m", "uvicorn", "routes_reference:app", "--host", "0.0.0.0", "--port", "8000", "--loop", "uvloop"]
