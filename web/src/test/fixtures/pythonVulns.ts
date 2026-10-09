// The python image's vulnerability summaries for latest and latest-dev,
// captured from the live Chainguard endpoints on 2026-10-09 (through this
// project's MCP server). Chainguard rebuilds images daily, so this won't match
// today's data; it pins the examples used in
// openspec/changes/add-security-fixes-chart (SF-1 to SF-8).

import type { PackageFixes, VulnReport } from '../../api'

const pf = (pkg: string, installed_version: string, fixed_count: number, not_affected_count: number, records_from?: string): PackageFixes => ({
  package: pkg,
  installed_version,
  fixed_count,
  not_affected_count,
  ...(records_from ? { records_from } : {}),
})

const note =
  'Based on the Wolfi security database, which records fixed and non-applicable vulnerabilities per package. ' +
  'It does not list unfixed vulnerabilities, so absence of a record does not prove an image is unaffected. ' +
  'Use a scanner such as grype for a full report.'

export const pythonVulnsLatest: VulnReport = {
  image: 'python',
  tag: 'latest',
  digest: 'sha256:6cc531a9ee4256bad0d3911da829906cb3c14c1d4aba9640004a94c0adf694e8',
  packages: [
    pf('brotli', '1.2.0-r5', 0, 2),
    pf('bzip2', '1.0.8-r25', 0, 0),
    pf('ca-certificates', '20260909-r2', 0, 0),
    pf('expat', '2.9.0-r0', 54, 4),
    pf('gcc', '16.2.0-r1', 1, 0),
    pf('gdbm', '1.26-r6', 0, 0),
    pf('glibc-2.44', '2.44-r8', 20, 12),
    pf('libffi', '3.8.0-r1', 0, 0),
    pf('mpdecimal', '4.0.1-r4', 0, 0),
    pf('ncurses', '6.6.20260926-r0', 0, 2),
    pf('openssl-4.0', '4.0.3-r5', 147, 6, 'openssl'),
    pf('openssl-provider-legacy-allowed', '4.0.2-r3', 0, 0),
    pf('py3-pip', '26.2.1-r3', 30, 9),
    pf('python-3.14', '3.14.8_git20261008-r0', 74, 4),
    pf('readline', '8.3-r3', 0, 0),
    pf('sqlite', '3.53.4-r2', 0, 4),
    pf('util-linux', '2.42.4-r0', 0, 4),
    pf('wolfi-baselayout', '20230201-r30', 0, 0),
    pf('xz', '5.8.4-r0', 0, 2),
    pf('zlib', '1.3.2.1_rc20260917-r0', 10, 2),
    pf('zstd', '1.5.7-r10', 0, 0),
  ],
  total_fixed: 336,
  total_not_affected: 51,
  note,
}

export const pythonVulnsLatestDev: VulnReport = {
  image: 'python',
  tag: 'latest-dev',
  digest: 'sha256:9876dbb802a6a7dc393f4cfe18a9325fb9e6190b95e9d29133ce596a4d893278',
  packages: [
    pf('apk-tools', '2.14.10-r17', 0, 0),
    pf('bash', '5.3-r14', 0, 0),
    pf('binutils', '2.47-r1', 52, 2),
    pf('brotli', '1.2.0-r5', 0, 2),
    pf('build-base', '1-r10', 0, 0),
    pf('busybox', '1.38.0-r3', 24, 8),
    pf('bzip2', '1.0.8-r25', 0, 0),
    pf('ca-certificates', '20260909-r2', 0, 0),
    pf('curl', '8.22.0-r4', 56, 4),
    pf('cyrus-sasl-heimdal', '2.1.28-r58', 0, 0),
    pf('e2fsprogs', '1.47.4-r2', 0, 0),
    pf('expat', '2.9.0-r0', 54, 4),
    pf('gcc', '16.2.0-r1', 1, 0),
    pf('gdbm', '1.26-r6', 0, 0),
    pf('git', '2.56.0-r0', 17, 1),
    pf('glibc-2.44', '2.44-r8', 20, 12),
    pf('gmp', '6.3.0-r10', 2, 0),
    pf('heimdal', '7.8.0-r52', 2, 0),
    pf('isl', '0.28-r3', 0, 0),
    pf('keyutils', '1.6.3-r40', 0, 0),
    pf('krb5', '1.22.2-r5', 0, 0),
    pf('krb5-conf', '1.0-r10', 0, 0),
    pf('libffi', '3.8.0-r1', 0, 0),
    pf('libidn2', '2.3.8-r9', 0, 4),
    pf('libpsl', '0.23.3-r1', 0, 0),
    pf('libunistring', '1.4.2-r3', 0, 0),
    pf('libverto', '0.3.2-r8', 0, 0),
    pf('libxcrypt', '4.5.2-r5', 0, 0),
    pf('linux-headers', '7.2.9-r0', 0, 0),
    pf('make', '4.4.1-r15', 0, 0),
    pf('mpc', '1.4.1-r1', 0, 0),
    pf('mpdecimal', '4.0.1-r4', 0, 0),
    pf('mpfr', '4.2.2-r3', 0, 0),
    pf('ncurses', '6.6.20260926-r0', 0, 2),
    pf('nghttp2', '1.70.0-r5', 5, 0),
    pf('nghttp3', '1.18.0-r1', 0, 0),
    pf('ngtcp2', '1.25.0-r5', 0, 0),
    pf('openldap-2.7', '2.7.1-r2', 0, 0),
    pf('openssf-compiler-options', '20250904-r11', 0, 0),
    pf('openssl-4.0', '4.0.3-r5', 147, 6, 'openssl'),
    pf('openssl-provider-legacy-allowed', '4.0.2-r3', 0, 0),
    pf('pcre2', '10.49-r1', 0, 4),
    pf('pkgconf', '3.0.8-r0', 2, 0),
    pf('posix-cc-wrappers', '2-r10', 0, 0),
    pf('py3-pip', '26.2.1-r3', 30, 9),
    pf('py3-setuptools', '84.0.0-r0', 6, 0),
    pf('python-3.14', '3.14.8_git20261008-r0', 74, 4),
    pf('readline', '8.3-r3', 0, 0),
    pf('sqlite', '3.53.4-r2', 0, 4),
    pf('util-linux', '2.42.4-r0', 0, 4),
    pf('uv', '0.12.24-r0', 34, 0),
    pf('wget', '1.25.0-r20', 10, 0),
    pf('wolfi-baselayout', '20230201-r30', 0, 0),
    pf('xz', '5.8.4-r0', 0, 2),
    pf('zlib', '1.3.2.1_rc20260917-r0', 10, 2),
    pf('zstd', '1.5.7-r10', 0, 0),
  ],
  total_fixed: 546,
  total_not_affected: 74,
  note,
}

/**
 * python latest with one fix not yet installed added to zlib. No live image had
 * one on 2026-10-09, so the pending-fix scenarios (SF-2.3, SF-3.2, SF-4.1) use this.
 */
export const pythonVulnsWithPending: VulnReport = {
  ...pythonVulnsLatest,
  packages: pythonVulnsLatest.packages?.map((p) => (p.package === 'zlib' ? { ...p, pending_fixes: ['CVE-2026-0001'] } : p)),
}
