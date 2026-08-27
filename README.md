# Joshua Kolden

**Seattle** · platform engineering · media production technology · pure Go · AI systems

> "What if we just didn't do that part?" I keep removing things until they work.

## What I'm working on

Open-source and standards work in media infrastructure, here and at [Avalanche-io](https://github.com/Avalanche-io). I architect, review and own the work; Claude Code accelerates the implementation.

- **[C4 Suite](https://github.com/Avalanche-io/c4)** (March 2026; latest release July 2026) — the reference implementations of [SMPTE ST 2114](https://ieeexplore.ieee.org/document/8187168), the content identifier standard, in Go, Python, TypeScript, Swift and C, co-versioned and released together through Homebrew, pip, npm, Swift Package Manager and GitHub Container Registry. Separate what a file *is* from where it lives — deduplication, provenance and asset tracking without a central authority. The files keep track of themselves. Includes c4sh, a shell that edits a filesystem as text, and c4git, a git clean/smudge filter that turns git into a media-asset version-control system.
- **[go-openexr](https://github.com/mrjoshuak/go-openexr)** (v1.1.0, February 2026) — pure-Go OpenEXR. No CGo, no C libraries, the format spec implemented from scratch. All 11 compression codecs including HTJ2K with progressive decode, deep data, multi-part files, tiled storage with mipmap/ripmap, multi-view. Output validated against OpenEXR's own `exrinfo` and `exrcheck`.
- **[gotio](https://github.com/Avalanche-io/gotio)** (December 2025) — pure-Go OpenTimelineIO, with adapters for Final Cut Pro 7 XML, Final Cut Pro X XML, CMX 3600 EDL, ALE, AAF (via bridge), HLS, GStreamer XGES and SVG.
- **[godoc-mcp](https://github.com/mrjoshuak/godoc-mcp)** — MCP server that gives coding agents structured Go documentation instead of dumped packages. 100+ stars. Because your copilot should actually be able to read the docs.
- **[absfs](https://github.com/absfs)** — abstract filesystem framework for Go. 36+ composable packages: in-memory, encrypted, cached, copy-on-write, union, NFS, S3, SFTP, FUSE, and more.

**More pure-Go format implementations** — complex binary formats without CGo:
- **[go-jpeg2000](https://github.com/mrjoshuak/go-jpeg2000)** — JPEG2000 with HTJ2K support
- **[go-alembic](https://github.com/mrjoshuak/go-alembic)** — Alembic 3D graphics interchange
- **[go-blosc](https://github.com/mrjoshuak/go-blosc)** — Blosc compression with SIMD-accelerated shuffle

<!-- Restore this paragraph only once the repos are pushed and public, with every name linked
     like the bullets above. Until then it stays out: an unlinked repo name on this page reads
     as work that does not exist.

**August 2026** — [temporal-media](https://github.com/mrjoshuak/temporal-media), a Temporal Go SDK media workflow — fan-out transcode activities with heartbeats and retries; [abr-pipeline](https://github.com/mrjoshuak/abr-pipeline), HLS-CMAF packaging in Go; [go-bwf](https://github.com/mrjoshuak/go-bwf), a BWF/ADM reader and BS.1770-4 loudness meter; and a single CI/CD pipeline for the C4 suite monorepo — merge queue, SLSA provenance, sigstore signing, SBOMs.
-->

## Previously

Sr. Manager, Microservices at Sphere Studios (Sphere Entertainment). Designed, built and ran the Kubernetes platform behind the studio's internal production systems — the pipelines moved from VM-based to container-native infrastructure-as-code, spinning up on demand, scaling with load, gone when done. Built and shipped a production AI agent integrated with the studio's internal issue tracker, code host and render queues: it reviewed issues, pulled repositories, generated fixes, opened pull requests, correlated tickets across systems and synthesized documentation. Delivered January 2025.

Before that, designed the C4 content identifier and carried it through SMPTE to publication as ST 2114:2017. The reference implementations were open-sourced through the Entertainment Technology Center at USC.

Film visual effects. Coined the term "virtual production" and was virtual production supervisor (uncredited) on the Avatar prototype — the virtual art department, virtual camera and Simulcam; the work was [publicly acknowledged by Fox and Lightstorm](https://www.hollywoodreporter.com/business/business-news/dispute-avatar-prodn-technology-settled-25310/). [34 film credits](https://www.imdb.com/name/nm0463978/).

## Talks

- **Keynote, Korea Media Festival 2026, Seoul** (June 2026) — "Everyday Work with Hollywood AI"
- **SMPTE Annual Technical Conference** — the C4 ID system, published as SMPTE ST 2114
- **[The Magic of C4](https://www.youtube.com/watch?v=vzh0JzKhY4o)** — ETC at USC, on indelible metadata and agreement without communication

---

Seattle, or remote. Platform and infrastructure work — joshua@avalanche.io
