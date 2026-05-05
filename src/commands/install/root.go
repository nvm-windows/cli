package install

type Root struct {
	Version     Install `cmd:"version" default:"withargs" help:"Install one or more versions of Node.js."`
	NativeTools Tools   `cmd:"native-tools" help:"Install native tools. This is the equivalent of running install_tools.bat. The full installation is multiple GB and may take 10-30 minutes to complete. NOT RECOMMENDED FOR MOST USERS. Only install this if you plan to compile native modules."`
}
