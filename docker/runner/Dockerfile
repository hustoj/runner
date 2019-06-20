FROM ubuntu
COPY runner /usr/bin/
RUN apt-get update -y && apt-get -y install openjdk-8-jdk --no-install-recommends && rm -rf /var/lib/apt/lists/*
VOLUME /data
VOLUME /var/log/runner
WORKDIR /data
CMD ["runner"]
