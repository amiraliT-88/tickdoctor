# VHS tape script for TickDoctor terminal demo
# Run with: vhs demo.tape

Output assets/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 720
Set Theme "Catppuccin Mocha"
Set Padding 20

# Command: Lint startup script
Type "tickdoctor lint --cmd 'java -Xms2G -Xmx16G -XX:+UseG1GC -XX:+UseParallelGC -jar server.jar' --ram 16G"
Sleep 500ms
Enter
Sleep 3s

# Command: Analyze with TUI
Type "clear"
Enter
Sleep 500ms

Type "tickdoctor analyze"
Sleep 500ms
Enter
Sleep 3s

# Navigate TUI tabs
Tab
Sleep 2s
Down 2
Sleep 1s
Down 2
Sleep 1s
Tab
Sleep 2s
Down
Sleep 2s
Tab
Sleep 2.5s

# Quit
Type "q"
Sleep 1s
