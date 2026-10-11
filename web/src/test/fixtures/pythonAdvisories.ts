// The python image's open advisories for latest and latest-dev, captured from
// the live endpoints on 2026-10-11 (through this project's API). Chainguard
// rebuilds images and publishes advisories daily, so this won't match today's
// data; it pins the examples used in openspec/changes/add-open-advisories
// (OA-5 to OA-9, OA-13). Statuses are Chainguard's current ones, read from its
// advisory feed (OSV.dev's copies of some glibc advisories were older and had
// them pending: latest has 2 open vulnerabilities, latest-dev 21).

import type { AdvisoryReport } from '../../api'

export const pythonAdvisoriesLatest: AdvisoryReport = {
  "image": "python",
  "tag": "latest",
  "digest": "sha256:daa185d3e927884a4f3d70af2a332cf84ccd5d9dc800e844b574738ac627ff0e",
  "checked_at": "2026-10-11T19:48:39Z",
  "source": "Known open advisories per Chainguard's published advisories, found via OSV.dev with their status from Chainguard's advisory feed, checked 2026-10-11 19:48 UTC.",
  "note": "Not yet assessed means detected but not yet assessed by Chainguard; some may turn out not to apply.",
  "total": 2,
  "counts": {
    "fix_available": 0,
    "pending_upstream_fix": 0,
    "confirmed": 0,
    "fix_not_planned": 0,
    "not_yet_assessed": 2
  },
  "vulnerabilities": [
    {
      "id": "CVE-2025-15367",
      "status": "not_yet_assessed",
      "aliases": [
        "GHSA-g82h-mgfp-jx8g"
      ],
      "packages": [
        {
          "name": "python-3.15",
          "version": "3.15.0_git20261010-r0",
          "status": "not_yet_assessed",
          "advisories": [
            "CGA-c5rh-xxpp-872r",
            "CGA-cjx6-763w-4wfc"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-12345",
      "status": "not_yet_assessed",
      "packages": [
        {
          "name": "python-3.15",
          "version": "3.15.0_git20261010-r0",
          "status": "not_yet_assessed",
          "advisories": [
            "CGA-2w26-qvh7-3jhg",
            "CGA-mp3h-w2rr-vq2f"
          ]
        }
      ]
    }
  ]
}

export const pythonAdvisoriesLatestDev: AdvisoryReport = {
  "image": "python",
  "tag": "latest-dev",
  "digest": "sha256:7ed9a26995f777888a056c2222b3c61b0eb175bf4c774dbed9851cd8a5c1f677",
  "checked_at": "2026-10-11T19:48:39Z",
  "source": "Known open advisories per Chainguard's published advisories, found via OSV.dev with their status from Chainguard's advisory feed, checked 2026-10-11 19:48 UTC.",
  "note": "Not yet assessed means detected but not yet assessed by Chainguard; some may turn out not to apply.",
  "total": 21,
  "counts": {
    "fix_available": 0,
    "pending_upstream_fix": 17,
    "confirmed": 0,
    "fix_not_planned": 0,
    "not_yet_assessed": 4
  },
  "vulnerabilities": [
    {
      "id": "CVE-2026-58470",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-5f52-px6m-c5hw"
      ],
      "packages": [
        {
          "name": "wget",
          "version": "1.25.0-r20",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-q3jg-48p9-vppw"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-58471",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-vv88-699v-w5rh"
      ],
      "packages": [
        {
          "name": "wget",
          "version": "1.25.0-r20",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-wcr9-rxgr-hrq5"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-58472",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-332r-8pmf-8m9p"
      ],
      "packages": [
        {
          "name": "wget",
          "version": "1.25.0-r20",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-4r22-ccq9-ch88"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-6844",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-p33j-xffm-gxj7"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-9gj8-gjxv-82qf",
            "CGA-fp47-6j6h-hj89"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-6845",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-chmq-57c3-4p7v"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-g89r-r3vh-pg54",
            "CGA-vm64-rhrg-jq27"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-90801",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-x22q-2wh5-h6hv"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-3x96-2xqq-vjp6",
            "CGA-9rcc-j9xg-72hj"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-90802",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-9587-j488-f7h3"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-52cx-9f2g-9fj3",
            "CGA-fw29-622p-rcr2"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-90803",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-fw55-q7rw-84f9"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-9c6v-m8x8-j3fw",
            "CGA-rfj3-3xpp-75fh"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-90804",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-45j6-5xqm-57mq"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-5gv6-97hw-c99g",
            "CGA-mxhv-p599-37pj"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-90828",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-fx9g-rhcq-63jg"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-7cxq-52mv-wf27",
            "CGA-m742-4p9j-3f58"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-90829",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-chxw-w34g-8w5c"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-59px-82vq-wxf5",
            "CGA-fxmw-q594-mwr3"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-90830",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-4xpf-8h39-fxrf"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-654r-h3j3-cp6j",
            "CGA-793r-8w27-7cpx"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-90831",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-fjw7-r654-jjmq"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-2v63-758j-xjm7",
            "CGA-qpvq-hw46-jg8f"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-91779",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-9ww6-c2m6-3277"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-84q2-7229-4h7w",
            "CGA-rqgg-6xvr-c6q9"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-91780",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-9vhp-r9qm-xf9x"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-3gg8-98rw-pxh5",
            "CGA-rcxm-9cq6-m5fx"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-91781",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-x33g-4rqc-v4wq"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-2wfw-w4p7-55x5",
            "CGA-4pq8-rj3p-hwfh"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-91782",
      "status": "pending_upstream_fix",
      "aliases": [
        "GHSA-vqfr-4h6g-3q8x"
      ],
      "packages": [
        {
          "name": "binutils",
          "version": "2.47-r1",
          "status": "pending_upstream_fix",
          "advisories": [
            "CGA-59hv-jrxx-977f",
            "CGA-r4h9-8rmm-9h9q"
          ]
        }
      ]
    },
    {
      "id": "CVE-2025-15367",
      "status": "not_yet_assessed",
      "aliases": [
        "GHSA-g82h-mgfp-jx8g"
      ],
      "packages": [
        {
          "name": "python-3.15",
          "version": "3.15.0_git20261010-r0",
          "status": "not_yet_assessed",
          "advisories": [
            "CGA-c5rh-xxpp-872r",
            "CGA-cjx6-763w-4wfc"
          ]
        }
      ]
    },
    {
      "id": "CVE-2025-47273",
      "status": "not_yet_assessed",
      "aliases": [
        "GHSA-5rjg-fvgr-3xxf"
      ],
      "packages": [
        {
          "name": "py3.15-pip-base",
          "version": "26.2.1-r4",
          "status": "not_yet_assessed",
          "advisories": [
            "CGA-84rc-65mm-8c9g",
            "CGA-m82f-jwmx-jq9w"
          ],
          "components": [
            {
              "name": "setuptools",
              "kind": "Python package"
            }
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-12345",
      "status": "not_yet_assessed",
      "packages": [
        {
          "name": "python-3.15",
          "version": "3.15.0_git20261010-r0",
          "status": "not_yet_assessed",
          "advisories": [
            "CGA-2w26-qvh7-3jhg",
            "CGA-mp3h-w2rr-vq2f"
          ]
        }
      ]
    },
    {
      "id": "CVE-2026-21895",
      "status": "not_yet_assessed",
      "aliases": [
        "GHSA-9c48-w39g-hm26"
      ],
      "packages": [
        {
          "name": "uv",
          "version": "0.13.0-r0",
          "status": "not_yet_assessed",
          "advisories": [
            "CGA-hv3q-j74h-9m29",
            "CGA-rx72-wfwm-8qwm"
          ],
          "components": [
            {
              "name": "rsa",
              "kind": "Rust crate"
            }
          ]
        }
      ]
    }
  ]
}
