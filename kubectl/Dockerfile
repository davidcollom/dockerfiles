ARG VERSION
FROM  --platform=${TARGETPLATFORM} rancher/k3s-upgrade:v$VERSION-k3s1

ENTRYPOINT ["/opt/k3s","kubectl"]
CMD ["--help"]
