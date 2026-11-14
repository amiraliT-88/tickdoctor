package flags

import "testing"

func FuzzParseCommandLine(f *testing.F) {
	f.Add("java -Xms4G -Xmx4G -XX:+UseG1GC -jar paper.jar")
	f.Add("java -Xmx8192M -XX:+AlwaysPreTouch -jar server.jar nogui")
	f.Add("-Xms2G -Xmx16G -XX:+UseConcMarkSweepGC -XX:MaxPermSize=256M")
	f.Add("corrupted command line with unmatched 'single quote and \"double quote")
	f.Add("")

	f.Fuzz(func(t *testing.T, cmd string) {
		_ = ParseCommandLine(cmd)
	})
}
