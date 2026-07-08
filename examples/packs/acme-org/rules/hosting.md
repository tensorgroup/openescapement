## Hosting & network exposure

- Production workloads run on the paved-road platform (https://platform.acme.example). POCs may use the allowed hosted platforms in the catalog below.
- Sharing a locally-hosted service: use the org tailnet (Tailscale — preferred) or Headscale for lab clusters. Cloudflare Tunnel requires review by #platform-team.
- Never expose a local service to the internet via raw port forwarding or by binding to 0.0.0.0 on a public interface.
- Any newly opened port on a deployed service requires review by the platform team before it ships.
