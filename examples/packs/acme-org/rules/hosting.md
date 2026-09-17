## Hosting & network exposure

- Run production workloads on the paved-road platform (https://platform.acme.example). For a POC, use one of the allowed hosted platforms in the catalog below.
- To share a locally hosted service, use the org tailnet (Tailscale). Use Headscale only for lab clusters. Cloudflare Tunnel requires review by #platform-team.
- Never expose a local service to the internet via raw port forwarding or by binding to 0.0.0.0 on a public interface.
- Any newly opened port on a deployed service requires review by #platform-team before it ships.
