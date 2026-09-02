# home-assistant

This image extends the official, version-pinned Home Assistant container and explicitly installs `ffmpeg`. I maintain it because camera, audio, and media integrations in my deployment need ffmpeg even when the chosen upstream image does not provide the required package layout.

Run it like the upstream Home Assistant container, including persistent `/config` storage and the host-network/device access required by your integrations.

```sh
docker run -d --name home-assistant --network host \
  -v home-assistant-config:/config \
  davidcollom/home-assistant:2025.1.2
```

This Dockerfile assumes an Alpine-based upstream tag because it rewrites APK repositories and installs via `apk`.
