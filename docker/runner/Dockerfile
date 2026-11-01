FROM ubuntu

RUN sed -i 's|archive.ubuntu.com|mirrors.aliyun.com|g' /etc/apt/sources.list
RUN apt-get update -y && apt-get -y install openjdk-8-jre-headless --no-install-recommends && rm -rf /var/lib/apt/lists/*
RUN useradd -m -u 1536 -s /bin/sh judger && mkdir -p /data /var/log/runner && chown -R judger:judger /data /var/log/runner
COPY runner /usr/bin/
VOLUME /data
VOLUME /var/log/runner
WORKDIR /data
USER judger
CMD ["runner"]
