# Catalog history

This branch holds the history of Chainguard's public image catalog, recorded by [chainguard-mcp](https://github.com/KenProle/chainguard-mcp) (unofficial; not affiliated with Chainguard). It contains no code: only `catalog-history.jsonl` and this file. The code lives on `main`.

A GitHub Action ([`history.yml`](https://github.com/KenProle/chainguard-mcp/blob/main/.github/workflows/history.yml)) adds one line every day at about 06:17 UTC by running `chainguard-mcp -record-history`. It reads the image list from the `images.chainguard.dev` sitemap and each image's free-tier status from Chainguard's anonymous token endpoint at `cgr.dev`.

## Format

`catalog-history.jsonl` has one JSON object per line, one line per snapshot, in date order.

| Field | Meaning |
|-------|---------|
| `date` | UTC date of the snapshot, `YYYY-MM-DD`. |
| `source` | `archive` for lines rebuilt from the Internet Archive's copies of the sitemap (one per month, April 2024 to October 2026), `live` for daily snapshots. |
| `total` | Number of images in the catalog. |
| `free` | Number of free-tier images, or `null` on `archive` lines (free-tier status was not recorded before the first live snapshot). |
| `added` | Images listed now but not in the previous snapshot. |
| `removed` | Images listed in the previous snapshot but not now. |
| `became_free` | Live lines after the first: images that are free now and were not free (or did not exist) before. |
| `became_subscription` | Live lines after the first: images that were free before and are subscription-only now (not images that were removed). |
| `unchecked` | Images whose free-tier check failed in this snapshot. Their status is treated as unchanged (a new image counts as not free), and they are never listed in `became_free` or `became_subscription`. |
| `images` | First line only: every image name. |
| `free_images` | First live line only: every free image (the free-tier baseline; these are not counted as `became_free`). |

All lists are sorted by name and are always present (`[]` when empty). Every line after the first holds only changes, so the full image list at any snapshot is the first line's `images` plus every later line's `added` and `removed`, up to that line; its length equals that line's `total`. The free set is rebuilt the same way from `free_images`, `became_free` and `became_subscription`.

Before the first live snapshot a line says only that an image was first seen *between* two captures: the Archive's captures are about a month apart or more.

## Rules

- At most one line per UTC date.
- A snapshot is refused (and the job fails, adding nothing) if the sitemap lists no images, or if it would remove more than 5% of the previous snapshot's images, since that indicates a broken sitemap rather than real retirements.
- Only the daily job writes here. Please don't rewrite or force-push this branch: the history can't be recovered.

## If the daily job stops

GitHub disables scheduled workflows in a public repository after 60 days without repository activity. If that happens, re-enable the **History** workflow under *Actions* on `main` (or run it by hand); the missing dates will show as a gap in the file.
