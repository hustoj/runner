FROM ubuntu
RUN apt-get update -y && apt-get -y install gcc g++ fp-compiler openjdk-8-jdk --no-install-recommends && rm -rf /var/lib/apt/lists/*
COPY compiler /usr/bin/
VOLUME /data
VOLUME /var/log/runner
WORKDIR /data
CMD ["compiler"]
