// Real API responses for the python image, captured from the live Chainguard
// endpoints on 2026-10-08 (through this project's Go server). Chainguard rebuilds
// images daily, so these won't match today's data; they pin the examples used
// in openspec/changes/add-variant-comparison (VC-5.1, VC-6.1, VC-7.1).

import type { ImageDetails, ImagePackages, ImageTags } from '../../api'

export const pythonTags: ImageTags = {
  image: 'python',
  reference: 'cgr.dev/chainguard/python',
  tags: [
    'latest',
    'latest-dev'
  ]
}

export const pythonDetails: Record<'latest' | 'latest-dev', ImageDetails> = {
  latest: {
    image: 'python',
    tag: 'latest',
    reference: 'cgr.dev/chainguard/python:latest',
    pinned_reference: 'cgr.dev/chainguard/python:latest@sha256:b6248c85ba9b97e1e61b30197f309cc4d21661f889fefa5268f0a7bc530dad46',
    digest: 'sha256:b6248c85ba9b97e1e61b30197f309cc4d21661f889fefa5268f0a7bc530dad46',
    created: '2026-10-08T02:44:58Z',
    source: 'https://github.com/chainguard-images/images/tree/main/images/python',
    platforms: [
      {
        platform: 'linux/amd64',
        digest: 'sha256:7d010bb252b8c00617ceec50af8e3d19a9b9f5faa3603c356f635aee98afd0f4',
        size_bytes: 26882525
      },
      {
        platform: 'linux/arm64',
        digest: 'sha256:ca95b8abc44cb142d96f772b8179cd2935b1988340c82521dff692fe44239140',
        size_bytes: 25182462
      }
    ],
    user: '65532',
    runs_as_root: false,
    entrypoint: [
      '/usr/bin/python'
    ],
    env: [
      'PATH=/usr/local/sbin:/usr/local/bin:/usr/bin:/usr/sbin:/sbin:/bin',
      'SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt'
    ],
    config_platform: 'linux/amd64'
  },
  'latest-dev': {
    image: 'python',
    tag: 'latest-dev',
    reference: 'cgr.dev/chainguard/python:latest-dev',
    pinned_reference: 'cgr.dev/chainguard/python:latest-dev@sha256:894aed3297d91283e1fc4c542f5374a4b5f3726134fda7c94eaa539342be1e05',
    digest: 'sha256:894aed3297d91283e1fc4c542f5374a4b5f3726134fda7c94eaa539342be1e05',
    created: '2026-10-08T02:44:58Z',
    source: 'https://github.com/chainguard-images/images/tree/main/images/python',
    platforms: [
      {
        platform: 'linux/amd64',
        digest: 'sha256:c028f42396e55365a33e9e5b8f36b0c18107ff14d30895ccf5ab5f5facd323cc',
        size_bytes: 272631719
      },
      {
        platform: 'linux/arm64',
        digest: 'sha256:780dd6a929ef11cf6762839076f8df3dab8e9f12d928d99046eacc15083ca1a4',
        size_bytes: 256387211
      }
    ],
    user: '65532',
    runs_as_root: false,
    entrypoint: [
      '/usr/bin/python'
    ],
    env: [
      'PATH=/usr/local/sbin:/usr/local/bin:/usr/bin:/usr/sbin:/sbin:/bin',
      'SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt'
    ],
    config_platform: 'linux/amd64'
  },
}

type Arch = 'amd64' | 'arm64'

export const pythonPackages: Record<'latest' | 'latest-dev', Record<Arch, ImagePackages>> = {
  latest: {
    amd64: {
      image: 'python',
      tag: 'latest',
      arch: 'amd64',
      digest: 'sha256:7d010bb252b8c00617ceec50af8e3d19a9b9f5faa3603c356f635aee98afd0f4',
      has_shell: false,
      has_apk: false,
      total: 29,
      packages: [
        {
          name: 'ca-certificates-bundle',
          version: '20260909-r2',
          origin: 'ca-certificates',
          license: 'MPL-2.0 AND MIT',
          distro: 'wolfi'
        },
        {
          name: 'gdbm',
          version: '1.26-r6',
          origin: 'gdbm',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'glibc-2.44',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'glibc-2.44-locale-posix',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'ld-linux-2.44',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libbrotlicommon1',
          version: '1.2.0-r5',
          origin: 'brotli',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libbrotlidec1',
          version: '1.2.0-r5',
          origin: 'brotli',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libbrotlienc1',
          version: '1.2.0-r5',
          origin: 'brotli',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libbz2-1',
          version: '1.0.8-r24',
          origin: 'bzip2',
          license: 'MPL-2.0 AND MIT',
          distro: 'wolfi'
        },
        {
          name: 'libexpat1',
          version: '2.9.0-r0',
          origin: 'expat',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libffi',
          version: '3.8.0-r1',
          origin: 'libffi',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libgcc',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libstdc++',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libuuid',
          version: '2.42.4-r0',
          origin: 'util-linux',
          license: 'GPL-3.0-or-later AND GPL-2.0-or-later AND GPL-2.0-only AND GPL-1.0-only AND LGPL-2.1-or-later AND BSD-1-Clause AND BSD-2-Clause AND BSD-3-Clause AND BSD-4-Clause-UC AND MIT AND CC-PDDC',
          distro: 'wolfi'
        },
        {
          name: 'libzstd1',
          version: '1.5.7-r10',
          origin: 'zstd',
          license: 'BSD-2-Clause AND GPL-2.0-only',
          distro: 'wolfi'
        },
        {
          name: 'mpdecimal',
          version: '4.0.1-r4',
          origin: 'mpdecimal',
          license: 'BSD-2-Clause',
          distro: 'wolfi'
        },
        {
          name: 'ncurses',
          version: '6.6.20260926-r0',
          origin: 'ncurses',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'ncurses-terminfo-base',
          version: '6.6.20260926-r0',
          origin: 'ncurses',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'openssl-4.0-libcrypto',
          version: '4.0.3-r5',
          origin: 'openssl-4.0',
          license: 'Apache-2.0',
          distro: 'wolfi'
        },
        {
          name: 'openssl-4.0-libssl',
          version: '4.0.3-r5',
          origin: 'openssl-4.0',
          license: 'Apache-2.0',
          distro: 'wolfi'
        },
        {
          name: 'openssl-provider-legacy-allowed',
          version: '4.0.2-r3',
          origin: 'openssl-provider-legacy-allowed',
          license: 'Apache-2.0',
          distro: 'wolfi'
        },
        {
          name: 'py3-pip-wheel',
          version: '26.2.1-r2',
          origin: 'py3-pip',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'python-3.14',
          version: '3.14.8_git20261008-r0',
          origin: 'python-3.14',
          license: 'PSF-2.0',
          distro: 'wolfi'
        },
        {
          name: 'python-3.14-base',
          version: '3.14.8_git20261008-r0',
          origin: 'python-3.14',
          license: 'PSF-2.0',
          distro: 'wolfi'
        },
        {
          name: 'readline',
          version: '8.3-r3',
          origin: 'readline',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'sqlite-libs',
          version: '3.53.4-r2',
          origin: 'sqlite',
          license: 'blessing',
          distro: 'wolfi'
        },
        {
          name: 'wolfi-baselayout',
          version: '20230201-r30',
          origin: 'wolfi-baselayout',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'xz',
          version: '5.8.4-r0',
          origin: 'xz',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'zlib',
          version: '1.3.2.1_rc20260917-r0',
          origin: 'zlib',
          license: 'MPL-2.0 AND MIT',
          distro: 'wolfi'
        }
      ]
    },
    arm64: {
      image: 'python',
      tag: 'latest',
      arch: 'arm64',
      digest: 'sha256:ca95b8abc44cb142d96f772b8179cd2935b1988340c82521dff692fe44239140',
      has_shell: false,
      has_apk: false,
      total: 29,
      packages: [
        {
          name: 'ca-certificates-bundle',
          version: '20260909-r2',
          origin: 'ca-certificates',
          license: 'MPL-2.0 AND MIT',
          distro: 'wolfi'
        },
        {
          name: 'gdbm',
          version: '1.26-r6',
          origin: 'gdbm',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'glibc-2.44',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'glibc-2.44-locale-posix',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'ld-linux-2.44',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libbrotlicommon1',
          version: '1.2.0-r5',
          origin: 'brotli',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libbrotlidec1',
          version: '1.2.0-r5',
          origin: 'brotli',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libbrotlienc1',
          version: '1.2.0-r5',
          origin: 'brotli',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libbz2-1',
          version: '1.0.8-r24',
          origin: 'bzip2',
          license: 'MPL-2.0 AND MIT',
          distro: 'wolfi'
        },
        {
          name: 'libexpat1',
          version: '2.9.0-r0',
          origin: 'expat',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libffi',
          version: '3.8.0-r1',
          origin: 'libffi',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libgcc',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libstdc++',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libuuid',
          version: '2.42.4-r0',
          origin: 'util-linux',
          license: 'GPL-3.0-or-later AND GPL-2.0-or-later AND GPL-2.0-only AND GPL-1.0-only AND LGPL-2.1-or-later AND BSD-1-Clause AND BSD-2-Clause AND BSD-3-Clause AND BSD-4-Clause-UC AND MIT AND CC-PDDC',
          distro: 'wolfi'
        },
        {
          name: 'libzstd1',
          version: '1.5.7-r10',
          origin: 'zstd',
          license: 'BSD-2-Clause AND GPL-2.0-only',
          distro: 'wolfi'
        },
        {
          name: 'mpdecimal',
          version: '4.0.1-r4',
          origin: 'mpdecimal',
          license: 'BSD-2-Clause',
          distro: 'wolfi'
        },
        {
          name: 'ncurses',
          version: '6.6.20260926-r0',
          origin: 'ncurses',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'ncurses-terminfo-base',
          version: '6.6.20260926-r0',
          origin: 'ncurses',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'openssl-4.0-libcrypto',
          version: '4.0.3-r5',
          origin: 'openssl-4.0',
          license: 'Apache-2.0',
          distro: 'wolfi'
        },
        {
          name: 'openssl-4.0-libssl',
          version: '4.0.3-r5',
          origin: 'openssl-4.0',
          license: 'Apache-2.0',
          distro: 'wolfi'
        },
        {
          name: 'openssl-provider-legacy-allowed',
          version: '4.0.2-r3',
          origin: 'openssl-provider-legacy-allowed',
          license: 'Apache-2.0',
          distro: 'wolfi'
        },
        {
          name: 'py3-pip-wheel',
          version: '26.2.1-r2',
          origin: 'py3-pip',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'python-3.14',
          version: '3.14.8_git20261008-r0',
          origin: 'python-3.14',
          license: 'PSF-2.0',
          distro: 'wolfi'
        },
        {
          name: 'python-3.14-base',
          version: '3.14.8_git20261008-r0',
          origin: 'python-3.14',
          license: 'PSF-2.0',
          distro: 'wolfi'
        },
        {
          name: 'readline',
          version: '8.3-r3',
          origin: 'readline',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'sqlite-libs',
          version: '3.53.4-r2',
          origin: 'sqlite',
          license: 'blessing',
          distro: 'wolfi'
        },
        {
          name: 'wolfi-baselayout',
          version: '20230201-r30',
          origin: 'wolfi-baselayout',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'xz',
          version: '5.8.4-r0',
          origin: 'xz',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'zlib',
          version: '1.3.2.1_rc20260917-r0',
          origin: 'zlib',
          license: 'MPL-2.0 AND MIT',
          distro: 'wolfi'
        }
      ]
    },
  },
  'latest-dev': {
    amd64: {
      image: 'python',
      tag: 'latest-dev',
      arch: 'amd64',
      digest: 'sha256:c028f42396e55365a33e9e5b8f36b0c18107ff14d30895ccf5ab5f5facd323cc',
      has_shell: true,
      has_apk: true,
      total: 76,
      packages: [
        {
          name: 'apk-tools',
          version: '2.14.10-r17',
          origin: 'apk-tools',
          license: 'GPL-2.0-only',
          distro: 'wolfi'
        },
        {
          name: 'bash',
          version: '5.3-r13',
          origin: 'bash',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'binutils',
          version: '2.47-r1',
          origin: 'binutils',
          license: 'GPL-2.0',
          distro: 'wolfi'
        },
        {
          name: 'build-base',
          version: '1-r9',
          origin: 'build-base',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'busybox',
          version: '1.38.0-r2',
          origin: 'busybox',
          license: 'GPL-2.0-only',
          distro: 'wolfi'
        },
        {
          name: 'ca-certificates-bundle',
          version: '20260909-r2',
          origin: 'ca-certificates',
          license: 'MPL-2.0 AND MIT',
          distro: 'wolfi'
        },
        {
          name: 'cyrus-sasl-heimdal-libs',
          version: '2.1.28-r58',
          origin: 'cyrus-sasl-heimdal',
          license: 'BSD-3-Clause',
          distro: 'wolfi'
        },
        {
          name: 'gcc',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'gdbm',
          version: '1.26-r6',
          origin: 'gdbm',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'git',
          version: '2.56.0-r0',
          origin: 'git',
          license: 'GPL-2.0-or-later AND ( Artistic-1.0-Perl OR GPL-1.0-or-later )',
          distro: 'wolfi'
        },
        {
          name: 'glibc-2.44',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'glibc-2.44-dev',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'glibc-2.44-locale-posix',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'gmp',
          version: '6.3.0-r10',
          origin: 'gmp',
          license: 'LGPL-3.0-or-later OR GPL-2.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'heimdal-libs',
          version: '7.8.0-r52',
          origin: 'heimdal',
          license: 'BSD-3-Clause',
          distro: 'wolfi'
        },
        {
          name: 'isl',
          version: '0.28-r3',
          origin: 'isl',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'keyutils-libs',
          version: '1.6.3-r40',
          origin: 'keyutils',
          license: 'GPL-2.0-or-later OR LGPL-2.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'krb5-conf',
          version: '1.0-r10',
          origin: 'krb5-conf',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'krb5-libs',
          version: '1.22.2-r5',
          origin: 'krb5',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'ld-linux-2.44',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libatomic',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libbrotlicommon1',
          version: '1.2.0-r5',
          origin: 'brotli',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libbrotlidec1',
          version: '1.2.0-r5',
          origin: 'brotli',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libbrotlienc1',
          version: '1.2.0-r5',
          origin: 'brotli',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libbz2-1',
          version: '1.0.8-r24',
          origin: 'bzip2',
          license: 'MPL-2.0 AND MIT',
          distro: 'wolfi'
        },
        {
          name: 'libcom_err',
          version: '1.47.4-r2',
          origin: 'e2fsprogs',
          license: 'GPL-2.0-or-later AND LGPL-2.0-or-later AND BSD-3-Clause AND MIT',
          distro: 'wolfi'
        },
        {
          name: 'libcrypt1-2.44',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libcurl-openssl4',
          version: '8.22.0-r4',
          origin: 'curl',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libexpat1',
          version: '2.9.0-r0',
          origin: 'expat',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libffi',
          version: '3.8.0-r1',
          origin: 'libffi',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libgcc',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libgomp',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libidn2',
          version: '2.3.8-r9',
          origin: 'libidn2',
          license: 'GPL-2.0-or-later AND LGPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libldap-2.7',
          version: '2.7.1-r2',
          origin: 'openldap-2.7',
          license: 'OLDAP-2.8',
          distro: 'wolfi'
        },
        {
          name: 'libnghttp2-14',
          version: '1.70.0-r5',
          origin: 'nghttp2',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libpcre2-8-0',
          version: '10.49-r1',
          origin: 'pcre2',
          license: 'BSD-3-Clause',
          distro: 'wolfi'
        },
        {
          name: 'libpsl',
          version: '0.23.3-r1',
          origin: 'libpsl',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libquadmath',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libstdc++',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libstdc++-dev',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libunistring',
          version: '1.4.2-r3',
          origin: 'libunistring',
          license: 'GPL-2.0-or-later OR LGPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libuuid',
          version: '2.42.4-r0',
          origin: 'util-linux',
          license: 'GPL-3.0-or-later AND GPL-2.0-or-later AND GPL-2.0-only AND GPL-1.0-only AND LGPL-2.1-or-later AND BSD-1-Clause AND BSD-2-Clause AND BSD-3-Clause AND BSD-4-Clause-UC AND MIT AND CC-PDDC',
          distro: 'wolfi'
        },
        {
          name: 'libverto',
          version: '0.3.2-r8',
          origin: 'libverto',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libxcrypt',
          version: '4.5.2-r5',
          origin: 'libxcrypt',
          license: 'GPL-2.0-or-later AND LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libxcrypt-dev',
          version: '4.5.2-r5',
          origin: 'libxcrypt',
          license: 'GPL-2.0-or-later AND LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libzstd1',
          version: '1.5.7-r10',
          origin: 'zstd',
          license: 'BSD-2-Clause AND GPL-2.0-only',
          distro: 'wolfi'
        },
        {
          name: 'linux-headers',
          version: '7.2.9-r0',
          origin: 'linux-headers',
          license: 'GPL-2.0-only WITH Linux-syscall-note',
          distro: 'wolfi'
        },
        {
          name: 'make',
          version: '4.4.1-r15',
          origin: 'make',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'mpc',
          version: '1.4.1-r1',
          origin: 'mpc',
          license: 'LGPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'mpdecimal',
          version: '4.0.1-r4',
          origin: 'mpdecimal',
          license: 'BSD-2-Clause',
          distro: 'wolfi'
        },
        {
          name: 'mpfr',
          version: '4.2.2-r3',
          origin: 'mpfr',
          license: 'LGPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'ncurses',
          version: '6.6.20260926-r0',
          origin: 'ncurses',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'ncurses-terminfo-base',
          version: '6.6.20260926-r0',
          origin: 'ncurses',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'nghttp3',
          version: '1.18.0-r1',
          origin: 'nghttp3',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'ngtcp2',
          version: '1.25.0-r5',
          origin: 'ngtcp2',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'openssf-compiler-options',
          version: '20250904-r11',
          origin: 'openssf-compiler-options',
          license: 'CC-BY-4.0',
          distro: 'wolfi'
        },
        {
          name: 'openssl-4.0-libcrypto',
          version: '4.0.3-r5',
          origin: 'openssl-4.0',
          license: 'Apache-2.0',
          distro: 'wolfi'
        },
        {
          name: 'openssl-4.0-libssl',
          version: '4.0.3-r5',
          origin: 'openssl-4.0',
          license: 'Apache-2.0',
          distro: 'wolfi'
        },
        {
          name: 'openssl-provider-legacy-allowed',
          version: '4.0.2-r3',
          origin: 'openssl-provider-legacy-allowed',
          license: 'Apache-2.0',
          distro: 'wolfi'
        },
        {
          name: 'pkgconf',
          version: '3.0.7-r0',
          origin: 'pkgconf',
          license: 'ISC',
          distro: 'wolfi'
        },
        {
          name: 'posix-cc-wrappers',
          version: '2-r10',
          origin: 'posix-cc-wrappers',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'py3-pip-wheel',
          version: '26.2.1-r2',
          origin: 'py3-pip',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'py3.14-pip',
          version: '26.2.1-r2',
          origin: 'py3-pip',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'py3.14-pip-base',
          version: '26.2.1-r2',
          origin: 'py3-pip',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'py3.14-setuptools',
          version: '84.0.0-r0',
          origin: 'py3-setuptools',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'python-3.14',
          version: '3.14.8_git20261008-r0',
          origin: 'python-3.14',
          license: 'PSF-2.0',
          distro: 'wolfi'
        },
        {
          name: 'python-3.14-base',
          version: '3.14.8_git20261008-r0',
          origin: 'python-3.14',
          license: 'PSF-2.0',
          distro: 'wolfi'
        },
        {
          name: 'python-3.14-base-dev',
          version: '3.14.8_git20261008-r0',
          origin: 'python-3.14',
          license: 'PSF-2.0',
          distro: 'wolfi'
        },
        {
          name: 'python-3.14-dev',
          version: '3.14.8_git20261008-r0',
          origin: 'python-3.14',
          license: 'PSF-2.0',
          distro: 'wolfi'
        },
        {
          name: 'readline',
          version: '8.3-r3',
          origin: 'readline',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'sqlite-libs',
          version: '3.53.4-r2',
          origin: 'sqlite',
          license: 'blessing',
          distro: 'wolfi'
        },
        {
          name: 'uv',
          version: '0.12.23-r2',
          origin: 'uv',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'wget',
          version: '1.25.0-r20',
          origin: 'wget',
          license: 'GPL-3.0',
          distro: 'wolfi'
        },
        {
          name: 'wolfi-baselayout',
          version: '20230201-r30',
          origin: 'wolfi-baselayout',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'xz',
          version: '5.8.4-r0',
          origin: 'xz',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'zlib',
          version: '1.3.2.1_rc20260917-r0',
          origin: 'zlib',
          license: 'MPL-2.0 AND MIT',
          distro: 'wolfi'
        }
      ]
    },
    arm64: {
      image: 'python',
      tag: 'latest-dev',
      arch: 'arm64',
      digest: 'sha256:780dd6a929ef11cf6762839076f8df3dab8e9f12d928d99046eacc15083ca1a4',
      has_shell: true,
      has_apk: true,
      total: 76,
      packages: [
        {
          name: 'apk-tools',
          version: '2.14.10-r17',
          origin: 'apk-tools',
          license: 'GPL-2.0-only',
          distro: 'wolfi'
        },
        {
          name: 'bash',
          version: '5.3-r13',
          origin: 'bash',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'binutils',
          version: '2.47-r1',
          origin: 'binutils',
          license: 'GPL-2.0',
          distro: 'wolfi'
        },
        {
          name: 'build-base',
          version: '1-r9',
          origin: 'build-base',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'busybox',
          version: '1.38.0-r2',
          origin: 'busybox',
          license: 'GPL-2.0-only',
          distro: 'wolfi'
        },
        {
          name: 'ca-certificates-bundle',
          version: '20260909-r2',
          origin: 'ca-certificates',
          license: 'MPL-2.0 AND MIT',
          distro: 'wolfi'
        },
        {
          name: 'cyrus-sasl-heimdal-libs',
          version: '2.1.28-r58',
          origin: 'cyrus-sasl-heimdal',
          license: 'BSD-3-Clause',
          distro: 'wolfi'
        },
        {
          name: 'gcc',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'gdbm',
          version: '1.26-r6',
          origin: 'gdbm',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'git',
          version: '2.56.0-r0',
          origin: 'git',
          license: 'GPL-2.0-or-later AND ( Artistic-1.0-Perl OR GPL-1.0-or-later )',
          distro: 'wolfi'
        },
        {
          name: 'glibc-2.44',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'glibc-2.44-dev',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'glibc-2.44-locale-posix',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'gmp',
          version: '6.3.0-r10',
          origin: 'gmp',
          license: 'LGPL-3.0-or-later OR GPL-2.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'heimdal-libs',
          version: '7.8.0-r52',
          origin: 'heimdal',
          license: 'BSD-3-Clause',
          distro: 'wolfi'
        },
        {
          name: 'isl',
          version: '0.28-r3',
          origin: 'isl',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'keyutils-libs',
          version: '1.6.3-r40',
          origin: 'keyutils',
          license: 'GPL-2.0-or-later OR LGPL-2.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'krb5-conf',
          version: '1.0-r10',
          origin: 'krb5-conf',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'krb5-libs',
          version: '1.22.2-r5',
          origin: 'krb5',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'ld-linux-2.44',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libatomic',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libbrotlicommon1',
          version: '1.2.0-r5',
          origin: 'brotli',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libbrotlidec1',
          version: '1.2.0-r5',
          origin: 'brotli',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libbrotlienc1',
          version: '1.2.0-r5',
          origin: 'brotli',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libbz2-1',
          version: '1.0.8-r24',
          origin: 'bzip2',
          license: 'MPL-2.0 AND MIT',
          distro: 'wolfi'
        },
        {
          name: 'libcom_err',
          version: '1.47.4-r2',
          origin: 'e2fsprogs',
          license: 'GPL-2.0-or-later AND LGPL-2.0-or-later AND BSD-3-Clause AND MIT',
          distro: 'wolfi'
        },
        {
          name: 'libcrypt1-2.44',
          version: '2.44-r8',
          origin: 'glibc-2.44',
          license: 'LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libcurl-openssl4',
          version: '8.22.0-r4',
          origin: 'curl',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libexpat1',
          version: '2.9.0-r0',
          origin: 'expat',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libffi',
          version: '3.8.0-r1',
          origin: 'libffi',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libgcc',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libgomp',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libidn2',
          version: '2.3.8-r9',
          origin: 'libidn2',
          license: 'GPL-2.0-or-later AND LGPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libldap-2.7',
          version: '2.7.1-r2',
          origin: 'openldap-2.7',
          license: 'OLDAP-2.8',
          distro: 'wolfi'
        },
        {
          name: 'libnghttp2-14',
          version: '1.70.0-r5',
          origin: 'nghttp2',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libpcre2-8-0',
          version: '10.49-r1',
          origin: 'pcre2',
          license: 'BSD-3-Clause',
          distro: 'wolfi'
        },
        {
          name: 'libpsl',
          version: '0.23.3-r1',
          origin: 'libpsl',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libquadmath',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libstdc++',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libstdc++-dev',
          version: '16.2.0-r1',
          origin: 'gcc',
          license: 'GPL-3.0-or-later WITH GCC-exception-3.1',
          distro: 'wolfi'
        },
        {
          name: 'libunistring',
          version: '1.4.2-r3',
          origin: 'libunistring',
          license: 'GPL-2.0-or-later OR LGPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libuuid',
          version: '2.42.4-r0',
          origin: 'util-linux',
          license: 'GPL-3.0-or-later AND GPL-2.0-or-later AND GPL-2.0-only AND GPL-1.0-only AND LGPL-2.1-or-later AND BSD-1-Clause AND BSD-2-Clause AND BSD-3-Clause AND BSD-4-Clause-UC AND MIT AND CC-PDDC',
          distro: 'wolfi'
        },
        {
          name: 'libverto',
          version: '0.3.2-r8',
          origin: 'libverto',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'libxcrypt',
          version: '4.5.2-r5',
          origin: 'libxcrypt',
          license: 'GPL-2.0-or-later AND LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libxcrypt-dev',
          version: '4.5.2-r5',
          origin: 'libxcrypt',
          license: 'GPL-2.0-or-later AND LGPL-2.1-or-later',
          distro: 'wolfi'
        },
        {
          name: 'libzstd1',
          version: '1.5.7-r10',
          origin: 'zstd',
          license: 'BSD-2-Clause AND GPL-2.0-only',
          distro: 'wolfi'
        },
        {
          name: 'linux-headers',
          version: '7.2.9-r0',
          origin: 'linux-headers',
          license: 'GPL-2.0-only WITH Linux-syscall-note',
          distro: 'wolfi'
        },
        {
          name: 'make',
          version: '4.4.1-r15',
          origin: 'make',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'mpc',
          version: '1.4.1-r1',
          origin: 'mpc',
          license: 'LGPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'mpdecimal',
          version: '4.0.1-r4',
          origin: 'mpdecimal',
          license: 'BSD-2-Clause',
          distro: 'wolfi'
        },
        {
          name: 'mpfr',
          version: '4.2.2-r3',
          origin: 'mpfr',
          license: 'LGPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'ncurses',
          version: '6.6.20260926-r0',
          origin: 'ncurses',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'ncurses-terminfo-base',
          version: '6.6.20260926-r0',
          origin: 'ncurses',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'nghttp3',
          version: '1.18.0-r1',
          origin: 'nghttp3',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'ngtcp2',
          version: '1.25.0-r5',
          origin: 'ngtcp2',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'openssf-compiler-options',
          version: '20250904-r11',
          origin: 'openssf-compiler-options',
          license: 'CC-BY-4.0',
          distro: 'wolfi'
        },
        {
          name: 'openssl-4.0-libcrypto',
          version: '4.0.3-r5',
          origin: 'openssl-4.0',
          license: 'Apache-2.0',
          distro: 'wolfi'
        },
        {
          name: 'openssl-4.0-libssl',
          version: '4.0.3-r5',
          origin: 'openssl-4.0',
          license: 'Apache-2.0',
          distro: 'wolfi'
        },
        {
          name: 'openssl-provider-legacy-allowed',
          version: '4.0.2-r3',
          origin: 'openssl-provider-legacy-allowed',
          license: 'Apache-2.0',
          distro: 'wolfi'
        },
        {
          name: 'pkgconf',
          version: '3.0.7-r0',
          origin: 'pkgconf',
          license: 'ISC',
          distro: 'wolfi'
        },
        {
          name: 'posix-cc-wrappers',
          version: '2-r10',
          origin: 'posix-cc-wrappers',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'py3-pip-wheel',
          version: '26.2.1-r2',
          origin: 'py3-pip',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'py3.14-pip',
          version: '26.2.1-r2',
          origin: 'py3-pip',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'py3.14-pip-base',
          version: '26.2.1-r2',
          origin: 'py3-pip',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'py3.14-setuptools',
          version: '84.0.0-r0',
          origin: 'py3-setuptools',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'python-3.14',
          version: '3.14.8_git20261008-r0',
          origin: 'python-3.14',
          license: 'PSF-2.0',
          distro: 'wolfi'
        },
        {
          name: 'python-3.14-base',
          version: '3.14.8_git20261008-r0',
          origin: 'python-3.14',
          license: 'PSF-2.0',
          distro: 'wolfi'
        },
        {
          name: 'python-3.14-base-dev',
          version: '3.14.8_git20261008-r0',
          origin: 'python-3.14',
          license: 'PSF-2.0',
          distro: 'wolfi'
        },
        {
          name: 'python-3.14-dev',
          version: '3.14.8_git20261008-r0',
          origin: 'python-3.14',
          license: 'PSF-2.0',
          distro: 'wolfi'
        },
        {
          name: 'readline',
          version: '8.3-r3',
          origin: 'readline',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'sqlite-libs',
          version: '3.53.4-r2',
          origin: 'sqlite',
          license: 'blessing',
          distro: 'wolfi'
        },
        {
          name: 'uv',
          version: '0.12.23-r2',
          origin: 'uv',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'wget',
          version: '1.25.0-r20',
          origin: 'wget',
          license: 'GPL-3.0',
          distro: 'wolfi'
        },
        {
          name: 'wolfi-baselayout',
          version: '20230201-r30',
          origin: 'wolfi-baselayout',
          license: 'MIT',
          distro: 'wolfi'
        },
        {
          name: 'xz',
          version: '5.8.4-r0',
          origin: 'xz',
          license: 'GPL-3.0-or-later',
          distro: 'wolfi'
        },
        {
          name: 'zlib',
          version: '1.3.2.1_rc20260917-r0',
          origin: 'zlib',
          license: 'MPL-2.0 AND MIT',
          distro: 'wolfi'
        }
      ]
    },
  },
}
