# Records docs/assets/demo.gif, docs/assets/overview.png and docs/assets/tui.png
# from made-up usage (docs/demo/setup.sh), never from a real ~/.omp.
# Run from the repository root: make demo

Output docs/assets/demo.gif

Require python3

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 820
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms
Set WindowBar Colorful
Set BorderRadius 10

Hide
Type `docs/demo/setup.sh /tmp/omp-cost-demo && export HOME=/tmp/omp-cost-demo PATH="$PWD/bin:$PATH" PS1='\[\e[38;5;147m\]~\[\e[0m\] $ ' && cd ~ && clear` Enter
Sleep 500ms
Show

# Plain output, not piped: a pipe drops the colors.
Type "omp-cost work 2026-10 --savings --no-sync" Sleep 400ms Enter
Sleep 3s
Screenshot docs/assets/overview.png
Sleep 1s

Type "clear" Enter
Type "omp-cost -m 2026-10 -i --no-sync" Sleep 400ms Enter
Sleep 2s
Screenshot docs/assets/tui.png
Type "]" Sleep 1500ms
Type "2" Sleep 1500ms
Type "4" Sleep 1500ms
Type "5" Sleep 2s
Type "q"
Sleep 1s
