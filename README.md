# NVM for Windows CLI (nvm.exe)

This is the `nvm.exe` source code. For details about using this, see the [official documentation](https://docs.nvm-windows.com).

This executable is part of several required for full Node.js verison management.

- `nvm.exe` (this repo) is responsible for downloads, (un)installs, caching, and configuration.
- `node.exe` ([shim/node](https://github.com/nvm-windows/shim/tree/main/node)) is the shim used to run Node.js.
- `proxy.exe` ([shim/proxy](https://github.com/nvm-windows/shim/tree/main/proxy)) is the global module shim (npm, npx, custom).
- `reshim.exe` ([shim/reshim](https://github.com/nvm-windows/shim/tree/main/reshim)) is a helper utility for syncing shims.
- `sync.exe` (private) is a closed source add-on app for identifying updates, releases, and fixes.

## Building from Source

This application uses [coreybutler](https://github.com/coreybutler)'s [qgo](https://github.com/quikdev/go) utility.

```powershell
git clone https://github.com/nvm-windows/nvm.git
cd .\src
qgo build
```

This will generate `.\bin\nvm.exe`.
