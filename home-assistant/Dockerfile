ARG VERSION
FROM homeassistant/home-assistant:${VERSION}

RUN <<-EOS
cat > /etc/apk/repositories << EOF; echo
https://dl-cdn.alpinelinux.org/alpine/v$(cut -d'.' -f1,2 /etc/alpine-release)/main/
https://dl-cdn.alpinelinux.org/alpine/v$(cut -d'.' -f1,2 /etc/alpine-release)/community/
EOF
EOS

RUN apk update
RUN apk add --update ffmpeg
