# 第一阶段：编译支持 Free Threading 的 Python 3.14
FROM debian:bookworm-slim AS builder

ENV PYTHON_VERSION=3.14.0
# 这里可以根据实际发布版本调整下载 URL
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
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
RUN wget -O python.tar.xz "${PYTHON_URL}" \
    && tar -xJf python.tar.xz \
    && cd Python-${PYTHON_VERSION} \
    # 启用 free threading (--disable-gil)
    && ./configure --enable-optimizations --disable-gil --prefix=/opt/python-3.14-t \
    && make -j$(nproc) \
    && make install

# 第二阶段：运行阶段
FROM debian:bookworm-slim

WORKDIR /app

# 从构建阶段复制编译好的 Python
COPY --from=builder /opt/python-3.14-t /opt/python-3.14-t
ENV PATH="/opt/python-3.14-t/bin:$PATH"

# 安装运行时的基础依赖（如 sqlite, ssl 等）
RUN apt-get update && apt-get install -y \
    libsqlite3-0 \
    libssl3 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
# 使用新编译的 pip 安装依赖
RUN pip3 install --no-cache-dir -r requirements.txt

COPY . .

# 赋予启动脚本执行权限
RUN chmod +x entrypoint.sh

EXPOSE 8000

ENTRYPOINT ["./entrypoint.sh"]
# 使用 python3.14t (free-threading 版本通常带有 t 后缀或独立路径)
# 强制使用 uvloop
CMD ["python3", "-m", "uvicorn", "routes_reference:app", "--host", "0.0.0.0", "--port", "8000", "--loop", "uvloop"]
