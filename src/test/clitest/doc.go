// Package clitest provides an isolated sandbox and Kong-based CLI harness for nvm command tests.
//
// Environment variables (optional):
//
//   - NVM_TEST_SIGNED_NODE: path to a signed node.exe copied by SeedVersion when set
//   - NVM_RESHIM_TEST_SYNC_HELPER: when "1", test binary acts as sync.exe stub (see syncstub.go)
//   - NVM_RESHIM_TEST_SYNC_RECORD: record sync invocation args to this file path
//   - NVM_RESHIM_TEST_SYNC_EXIT_CODE: exit code for the sync stub process
//   - NVM_TEST_SKIP_LICENSE: reserved for harness callers that skip license activation
//   - NVM_TEST_HTTP_FIXTURES: reserved for offline HTTP fixtures in install/list-remote tests
//
// Helpers:
//
//   - ExecuteBootstrapped: bootstrap + link mode for use/on/off tests (avoids async reshim.exe)
//   - ExecuteWithSyncStub: run commands that invoke sync.exe via the test-binary stub
//   - ApplyNodeMirrorFixture / SeedCacheArchive: offline list/install tests via test/testdata/index.tab
//
// Tests use HKCU\Software\NVMTest\... registry keys only — never HKLM.
package clitest
