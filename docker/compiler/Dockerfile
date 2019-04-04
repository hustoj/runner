FROM ubuntu
RUN apt-get update -y && apt-get -y install gcc g++
COPY compiler /usr/bin/
VOLUME /data
VOLUME /var/log/runner
WORKDIR /data
ENTRYPOINT ["compiler"]
