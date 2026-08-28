package cli

import (
	"fmt"
	"io"
)

func runCompletion(args []string, rt Runtime) error {
	if commandHelpRequested(args) {
		_, err := io.WriteString(rt.Stdout, completionHelp)
		return err
	}
	if len(args) == 0 {
		return &UsageError{Message: "shell is required"}
	}
	if len(args) > 1 {
		return &UsageError{Argument: args[1]}
	}

	script, ok := completionScripts[args[0]]
	if !ok {
		return &UsageError{Message: fmt.Sprintf("unsupported shell %q (choose bash, fish, or zsh)", args[0])}
	}
	_, err := io.WriteString(rt.Stdout, script)
	return err
}

var completionScripts = map[string]string{
	"bash": bashCompletion,
	"fish": fishCompletion,
	"zsh":  zshCompletion,
}

const bashCompletion = `# bash completion for forge
_forge_completion() {
    local current previous command
    current="${COMP_WORDS[COMP_CWORD]}"
    previous="${COMP_WORDS[COMP_CWORD-1]}"
    command="${COMP_WORDS[1]}"

    case "$previous" in
        --type)
            COMPREPLY=($(compgen -W "friction action follow-up decision" -- "$current"))
            return
            ;;
        --frequency)
            COMPREPLY=($(compgen -W "daily weekly monthly occasional unknown" -- "$current"))
            return
            ;;
        --impact)
            COMPREPLY=($(compgen -W "low medium high unknown" -- "$current"))
            return
            ;;
        --category)
            COMPREPLY=($(compgen -W "information-finding repeated-action context-switching remembering verification waiting other" -- "$current"))
            return
            ;;
    esac

    if [[ $COMP_CWORD -eq 1 ]]; then
        COMPREPLY=($(compgen -W "capture completion delete list show --help --version" -- "$current"))
        return
    fi

    case "$command" in
        capture)
            COMPREPLY=($(compgen -W "--category --current-workaround --frequency --help --impact --json --project --quick --type -h" -- "$current"))
            ;;
        completion)
            COMPREPLY=($(compgen -W "bash fish zsh --help -h" -- "$current"))
            ;;
        delete)
            COMPREPLY=($(compgen -W "--help -h" -- "$current"))
            ;;
        list)
            COMPREPLY=($(compgen -W "--help --json --limit --project --type -h" -- "$current"))
            ;;
        show)
            COMPREPLY=($(compgen -W "--help --json -h" -- "$current"))
            ;;
    esac
}
complete -F _forge_completion forge
`

const fishCompletion = `# fish completion for forge
complete -c forge -f
complete -c forge -n __fish_use_subcommand -s h -l help -d 'Show help'
complete -c forge -n __fish_use_subcommand -l version -d 'Show version'
complete -c forge -n __fish_use_subcommand -a capture -d 'Capture work'
complete -c forge -n __fish_use_subcommand -a completion -d 'Generate shell completion scripts'
complete -c forge -n __fish_use_subcommand -a delete -d 'Delete a capture'
complete -c forge -n __fish_use_subcommand -a list -d 'List captures'
complete -c forge -n __fish_use_subcommand -a show -d 'Show a capture'

complete -c forge -n '__fish_seen_subcommand_from capture' -s h -l help -d 'Show help'
complete -c forge -n '__fish_seen_subcommand_from capture' -l quick -d 'Capture without prompting'
complete -c forge -n '__fish_seen_subcommand_from capture' -l json -d 'Write JSON'
complete -c forge -n '__fish_seen_subcommand_from capture' -l type -r -a 'friction action follow-up decision' -d 'Set capture type'
complete -c forge -n '__fish_seen_subcommand_from capture' -l project -r -d 'Set friction project'
complete -c forge -n '__fish_seen_subcommand_from capture' -l frequency -r -a 'daily weekly monthly occasional unknown' -d 'Set friction frequency'
complete -c forge -n '__fish_seen_subcommand_from capture' -l impact -r -a 'low medium high unknown' -d 'Set friction impact'
complete -c forge -n '__fish_seen_subcommand_from capture' -l category -r -a 'information-finding repeated-action context-switching remembering verification waiting other' -d 'Set friction category'
complete -c forge -n '__fish_seen_subcommand_from capture' -l current-workaround -r -d 'Set friction workaround'

complete -c forge -n '__fish_seen_subcommand_from completion' -s h -l help -d 'Show help'
complete -c forge -n '__fish_seen_subcommand_from completion' -a 'bash fish zsh'

complete -c forge -n '__fish_seen_subcommand_from delete' -s h -l help -d 'Show help'

complete -c forge -n '__fish_seen_subcommand_from list' -s h -l help -d 'Show help'
complete -c forge -n '__fish_seen_subcommand_from list' -l json -d 'Write JSON'
complete -c forge -n '__fish_seen_subcommand_from list' -l limit -r -d 'Limit results'
complete -c forge -n '__fish_seen_subcommand_from list' -l project -r -d 'Filter by project'
complete -c forge -n '__fish_seen_subcommand_from list' -l type -r -a 'friction action follow-up decision' -d 'Filter by capture type'

complete -c forge -n '__fish_seen_subcommand_from show' -s h -l help -d 'Show help'
complete -c forge -n '__fish_seen_subcommand_from show' -l json -d 'Write JSON'
`

const zshCompletion = `#compdef forge

_forge() {
    local context state line
    typeset -A opt_args

    _arguments -C \
        '(-h --help)'{-h,--help}'[Show help]' \
        '--version[Show version]' \
        '1:command:((capture\:Capture\ work completion\:Generate\ shell\ completion\ scripts delete\:Delete\ a\ capture list\:List\ captures show\:Show\ a\ capture))' \
        '*::argument:->arguments'

    case "$words[2]" in
        capture)
            _arguments \
                '(-h --help)'{-h,--help}'[Show help]' \
                '--quick[Capture without prompting]' \
                '--json[Write JSON]' \
                '--type[Set capture type]:type:(friction action follow-up decision)' \
                '--project[Set friction project]:project:' \
                '--frequency[Set friction frequency]:frequency:(daily weekly monthly occasional unknown)' \
                '--impact[Set friction impact]:impact:(low medium high unknown)' \
                '--category[Set friction category]:category:(information-finding repeated-action context-switching remembering verification waiting other)' \
                '--current-workaround[Set friction workaround]:workaround:' \
                '1:description:'
            ;;
        completion)
            _arguments \
                '(-h --help)'{-h,--help}'[Show help]' \
                '1:shell:(bash fish zsh)'
            ;;
        delete)
            _arguments \
                '(-h --help)'{-h,--help}'[Show help]' \
                '1:record ID:'
            ;;
        list)
            _arguments \
                '(-h --help)'{-h,--help}'[Show help]' \
                '--json[Write JSON]' \
                '--limit[Limit results]:limit:' \
                '--project[Filter by project]:project:' \
                '--type[Filter by capture type]:type:(friction action follow-up decision)'
            ;;
        show)
            _arguments \
                '(-h --help)'{-h,--help}'[Show help]' \
                '--json[Write JSON]' \
                '1:record ID:'
            ;;
    esac
}

compdef _forge forge
`
