FROM ubuntu
COPY runner /usr/bin/
RUN apt-get update -y && apt-get -y install openjdk-8-jdk
VOLUME /data
VOLUME /var/log/runner
WORKDIR /data
CMD ["runner"]