FROM ubuntu:18.04

MAINTAINER david@collom.co.uk

ENV BASEDIR=/usr/lib/unifi \
  DATADIR=/var/lib/unifi \
  RUNDIR=/var/run/unifi \
  LOGDIR=/var/log/unifi \
  JVM_MAX_HEAP_SIZE=1024M \
  JVM_INIT_HEAP_SIZE=1024M \
  JAVA_HOME=/usr/lib/jvm/java-8-oracle

RUN apt-get update && \
    apt-get install -y software-properties-common apt-utils

# Accept Oracle License
RUN echo debconf shared/accepted-oracle-license-v1-1 select true |  debconf-set-selections && \
    echo debconf shared/accepted-oracle-license-v1-1 seen true |  debconf-set-selections

# Install Java
RUN add-apt-repository -y ppa:webupd8team/java && \
    apt-get -y --force-yes --no-install-recommends install oracle-java8-installer \
    oracle-java8-set-default \
    oracle-java8-installer \
    jsvc joe mc tcpdump lynx haveged net-tools

# Install mongodb
RUN  apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv BC711F9BA15703C6 \
  && echo "deb http://repo.mongodb.org/apt/ubuntu xenial/mongodb-org/3.4 multiverse" |  tee /etc/apt/sources.list.d/mongodb-org-3.0.list \
  && apt-get update && apt-get -q -y install \
    mongodb-org-server

#Install Unifi
RUN  apt-key adv --keyserver keyserver.ubuntu.com --recv 06E85760C0A52C50 \
  && echo "deb http://www.ubnt.com/downloads/unifi/debian stable ubiquiti" |  tee /etc/apt/sources.list.d/ubiquiti.list \
  && apt-get update && apt-get -q -y install unifi

# Pre-seed some of the data/directories etc..
RUN /etc/init.d/haveged restart; /etc/init.d/unifi restart

# Set Default command
CMD /etc/init.d/unifi start && tail -f /dev/null
EXPOSE 6789/tcp 8080/tcp 8443/tcp 8880/tcp 8843/tcp 3478/udp