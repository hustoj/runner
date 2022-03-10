FROM ubuntu

RUN sed -i 's|archive.ubuntu.com|mirrors.aliyun.com|g' /etc/apt/sources.list
RUN apt-get update -y && apt-get -y install openjdk-8-jre-headless --no-install-recommends && rm -rf /var/lib/apt/lists/*
COPY runner /usr/bin/
VOLUME /data
VOLUME /var/log/runner
WORKDIR /data
CMD ["runner"]
