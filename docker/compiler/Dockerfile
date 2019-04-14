FROM ubuntu
RUN apt-get update -y && apt-get -y install gcc g++ fpc openjdk-8-jdk
COPY compiler /usr/bin/
VOLUME /data
VOLUME /var/log/runner
WORKDIR /data
CMD ["compiler"]
