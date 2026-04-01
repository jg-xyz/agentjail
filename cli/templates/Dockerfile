FROM ubuntu:24.04 as base

ARG SHELL="bash"
ARG EDITOR="micro"
ARG FILE_BROWSER="rovr"
ARG USE_COPILOT="false"
ARG USE_OPENCODE="false"
ARG USE_CLAUDE_CODE="false"
ARG HOSTNAME="agentjail"
ARG STARSHIP_VERSION="1.24.2"
ENV STARSHIP_CONFIG="/root/.config/starship.toml"

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install basic dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    curl \
    gh \
    git \
    jq \
    micro \
    nano \
    python3 \
    python3-pip \
    python3-venv \
    unzip \
    vim \
    wget \
    zsh \
    && rm -rf /var/lib/apt/lists/*

# Pre-configure Docker's official apt repository so docker-ce-cli can be
# installed at runtime (when -P is used) without pulling in the full daemon.
RUN install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc && \
    chmod a+r /etc/apt/keyrings/docker.asc && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    | tee /etc/apt/sources.list.d/docker.list > /dev/null

ENV SHELL=${SHELL}
ENV FILE_BROWSER=${FILE_BROWSER}
ENV PATH="/root/.agentjail/bin:${PATH}"

# configure zsh
RUN [ "${SHELL}" = "zsh" ] && chsh -s $(which zsh)

# Install mise
RUN curl https://mise.run | sh
ENV PATH="/root/.local/bin:$PATH"
RUN echo 'eval "$(~/.local/bin/mise activate bash)"' >> ~/.bashrc && \
    echo 'eval "$(~/.local/bin/mise activate zsh)"' >> ~/.zshrc

# install uv
RUN curl -LsSf https://astral.sh/uv/install.sh | sh

# install tools
RUN mise use -g \
    aws-cli \
    bat \
    eza \
    fd \
    fzf \
    node \
    ripgrep \
    television \
    yq \
    zellij \
    pipx:rich-cli \
    pipx:rovr

#FROM base as agentjail
#RUN curl -sS https://starship.rs/install.sh | sh -s -- -p unknown-linux-gnu -a x86_64

#rm ~/.config/starship.toml && \

# Configure micro: unbind Ctrl+t which zellij intercepts for new-tab
RUN mkdir -p /root/.config/micro && \
    printf '{"CtrlT": "None"}\n' > /root/.config/micro/bindings.json

# install neovim (when selected as editor)
RUN if [ "${EDITOR}" = "nvim" ]; then \
    apt-get update && apt-get install -y neovim && rm -rf /var/lib/apt/lists/*; \
else \
    echo "Not installing neovim"; \
fi

# install helix (when selected as editor)
RUN if [ "${EDITOR}" = "hx" ]; then \
    apt-get update && apt-get install -y xz-utils && rm -rf /var/lib/apt/lists/* && \
    HELIX_RELEASE=$(curl -s https://api.github.com/repos/helix-editor/helix/releases/latest | jq -r '.tag_name') && \
    curl -L "https://github.com/helix-editor/helix/releases/download/${HELIX_RELEASE}/helix-${HELIX_RELEASE}-x86_64-linux.tar.xz" \
        -o helix.tar.xz && \
    tar xf helix.tar.xz && \
    mv "helix-${HELIX_RELEASE}-x86_64-linux/hx" /usr/local/bin/hx && \
    mkdir -p /usr/local/lib/helix && \
    mv "helix-${HELIX_RELEASE}-x86_64-linux/runtime" /usr/local/lib/helix/runtime && \
    rm -rf helix.tar.xz "helix-${HELIX_RELEASE}-x86_64-linux" && \
    printf '\nexport HELIX_RUNTIME=/usr/local/lib/helix/runtime\n' >> /root/.zshrc && \
    printf '\nexport HELIX_RUNTIME=/usr/local/lib/helix/runtime\n' >> /root/.bashrc; \
else \
    echo "Not installing helix"; \
fi

# install fresh (when selected as editor)
RUN if [ "${EDITOR}" = "fresh" ]; then \
    mise x -- npm install -g @fresh-editor/fresh-editor && \
    ln -sf "$(mise x -- npm prefix -g)/bin/fresh" /usr/local/bin/fresh; \
else \
    echo "Not installing fresh"; \
fi

# install yazi (when selected as file browser)
RUN if [ "${FILE_BROWSER}" = "yazi" ]; then \
    YAZI_VERSION=$(curl -s https://api.github.com/repos/sxyazi/yazi/releases/latest | jq -r '.tag_name') && \
    curl -L "https://github.com/sxyazi/yazi/releases/download/${YAZI_VERSION}/yazi-x86_64-unknown-linux-gnu.zip" \
        -o yazi.zip && \
    unzip yazi.zip && \
    mv "yazi-x86_64-unknown-linux-gnu/yazi" /usr/local/bin/yazi && \
    rm -rf yazi.zip "yazi-x86_64-unknown-linux-gnu"; \
else \
    echo "Not installing yazi"; \
fi

# install nnn (when selected as file browser)
RUN if [ "${FILE_BROWSER}" = "nnn" ]; then \
    apt-get update && apt-get install -y nnn && rm -rf /var/lib/apt/lists/*; \
else \
    echo "Not installing nnn"; \
fi

# install superfile/spf (when selected as file browser)
RUN if [ "${FILE_BROWSER}" = "spf" ]; then \
    SPF_VERSION=$(curl -s https://api.github.com/repos/yorukot/superfile/releases/latest | jq -r '.tag_name') && \
    curl -L "https://github.com/yorukot/superfile/releases/download/${SPF_VERSION}/superfile-linux-${SPF_VERSION}-amd64.tar.gz" \
        -o spf.tar.gz && \
    tar xf spf.tar.gz && \
    mv dist/spf /usr/local/bin/spf && \
    rm -rf spf.tar.gz dist; \
else \
    echo "Not installing superfile"; \
fi

# shell history configuration
# HISTFILE is injected at runtime by agentjail pointing to /root/.agentjail/<shell>_history
RUN cat >> ~/.zshrc <<'HISTCONF'
HISTSIZE=10000
SAVEHIST=10000
setopt INC_APPEND_HISTORY
setopt HIST_IGNORE_DUPS
HISTCONF

RUN cat >> ~/.bashrc <<'HISTCONF'
HISTSIZE=10000
HISTFILESIZE=10000
shopt -s histappend
PROMPT_COMMAND="${PROMPT_COMMAND:+${PROMPT_COMMAND}$'\n'}history -a"
HISTCONF

# aliases
RUN echo "alias files='\$FILE_BROWSER'" >> ~/.${SHELL}rc && \
    echo "alias edit='\$EDITOR'" >> ~/.${SHELL}rc && \
    echo "alias /exit=exit" >> ~/.${SHELL}rc && \
    echo "\n\n" >> ~/.${SHELL}rc

# When running inside a zellij session, override 'exit' to quit zellij
# (which also exits the container since zellij is launched with exec).
# The ZELLIJ env var is set by zellij itself, so this is a no-op outside it.
RUN cat >> ~/.${SHELL}rc <<'ZELLIJ_EXIT_HOOK'

if [ -n "${ZELLIJ}" ]; then
    alias exit='zellij action quit'
fi
ZELLIJ_EXIT_HOOK

# configure prompt
#RUN starship init ${SHELL} >> ~/.${SHELL}rc && \
#    wget https://github.com/mattmc3/zephyr/raw/refs/heads/main/plugins/prompt/themes/hydro.toml -O ~/.config/starship.toml

# install opencode
RUN [ "${USE_OPENCODE}" = "true" ] && curl -LsSf https://opencode.com/install.sh | sh || echo "Not installing opencode"

# install copilot
RUN [ "${USE_COPILOT}" = "true" ] && curl -fsSL https://gh.io/copilot-install | bash || echo "Not installing copilot"

# install claude code
RUN if [ "${USE_CLAUDE_CODE}" = "true" ]; then \
    mise x -- npm install -g @anthropic-ai/claude-code && \
    ln -sf "$(mise x -- npm prefix -g)/bin/claude" /usr/local/bin/claude; \
else \
    echo "Not installing claude code"; \
fi

# configure MOTD
RUN . /etc/os-release && \
    case "$ID" in \
      "ubuntu") \
        PKG_MANAGER="apt" ;; \
      "alpine") \
        PKG_MANAGER="apk" ;; \
      "fedora") \
        PKG_MANAGER="dnf" ;; \
      "debian") \
        PKG_MANAGER="apt" ;; \
      "arch") \
        PKG_MANAGER="pacman" ;; \
    esac && \
    AVAILABLE_AGENTS="Available Agents:" && \
    if [ "${USE_COPILOT}" = "true" ]; then \
      AVAILABLE_AGENTS="${AVAILABLE_AGENTS} [bold blue]Copilot[/bold blue]"; \
    fi && \
    if [ "${USE_OPENCODE}" = "true" ]; then \
      AVAILABLE_AGENTS="${AVAILABLE_AGENTS} [bold green]OpenCode[/bold green]"; \
    fi && \
    if [ "${USE_CLAUDE_CODE}" = "true" ]; then \
      AVAILABLE_AGENTS="${AVAILABLE_AGENTS} [bold orange3]Claude Code[/bold orange3]"; \
    fi && \
    AVAILABLE_EDITORS="Available Editors: vim, nano, micro" && \
    [ "${EDITOR}" = "nvim" ] && AVAILABLE_EDITORS="${AVAILABLE_EDITORS}, neovim" || true && \
    [ "${EDITOR}" = "hx" ] && AVAILABLE_EDITORS="${AVAILABLE_EDITORS}, helix" || true && \
    [ "${EDITOR}" = "fresh" ] && AVAILABLE_EDITORS="${AVAILABLE_EDITORS}, fresh" || true && \
    AVAILABLE_EDITORS="${AVAILABLE_EDITORS} (Primary: [bold blue]${EDITOR}[/bold blue])" && \
    ALIASES="Available Aliases: [blue][bold]\"files\"[/bold]: [green]${FILE_BROWSER}[/green], [bold]\"edit\"[/bold]: [green]${EDITOR}[/green][/blue]" && \
    printf "rich '[b]Welcome to [bold white]AgentJail![/bold white][/b]' --print --rule --rule-style 'bold blue' && rich 'You are now in a container running [bold gold3]$PRETTY_NAME[/bold gold3] \nYour project is mounted at [bold gold3]/project[/bold gold3] \nAvailable Package Managers: [bold dark_goldenrod]$PKG_MANAGER, mise, uv, pip, npm[/bold dark_goldenrod] \nDefault shell: [bold dark_goldenrod]$SHELL[/bold dark_goldenrod] \n \n$AVAILABLE_AGENTS \n \n$AVAILABLE_EDITORS \n \n$ALIASES \n\n type [red bold]exit[/red bold] to exit the container \n\nhave fun, and most importantly, \n[bold medium_orchid]~~ v i b e ~~[/bold medium_orchid]\n\n' --print --style 'blue'" >>  ~/.${SHELL}rc

# Fix file ownership on shell exit: chown /project back to the host user.
# HOST_UID and HOST_GID are injected by the agentjail CLI at runtime.
RUN cat >> ~/.zshrc <<'OWNERSHIP_FIX'

zshexit() {
    # Skip in zellij: the launcher wrapper handles chown once after the session ends.
    if [ -z "${ZELLIJ}" ] && [ -n "${HOST_UID}" ] && [ -n "${HOST_GID}" ]; then
        chown -R "${HOST_UID}:${HOST_GID}" /project /root/.agentjail 2>/dev/null || true
    fi
}
OWNERSHIP_FIX

RUN cat >> ~/.bashrc <<'OWNERSHIP_FIX'

# Skip in zellij: the launcher wrapper handles chown once after the session ends.
trap 'if [ -z "${ZELLIJ}" ] && [ -n "${HOST_UID}" ] && [ -n "${HOST_GID}" ]; then chown -R "${HOST_UID}:${HOST_GID}" /project /root/.agentjail 2>/dev/null || true; fi' EXIT
OWNERSHIP_FIX

# install starship
RUN wget https://github.com/starship/starship/releases/download/v${STARSHIP_VERSION}/starship-x86_64-unknown-linux-gnu.tar.gz && \
    tar xzvf starship-x86_64-unknown-linux-gnu.tar.gz && \
    mv starship /usr/local/bin/starship && \
    rm starship-x86_64-unknown-linux-gnu.tar.gz && \
    mkdir -p /root/.config && \
    wget https://github.com/mattmc3/zephyr/raw/refs/heads/main/plugins/prompt/themes/hydro.toml -O ${STARSHIP_CONFIG} && \
    sed -i 's/^format = .*/format = "$directory$character"/' ${STARSHIP_CONFIG} && \
    sed -i 's/^right_format = .*/right_format = "$git_branch$git_status"/' ${STARSHIP_CONFIG} && \
    sed -i 's/success_symbol = .*/success_symbol = "[❱❱](bold purple)"/' ${STARSHIP_CONFIG} && \
    sed -i 's/error_symbol = .*/error_symbol = "[❱❱](bold red)"/' ${STARSHIP_CONFIG} && \
    sed -i 's/style = "blue"/style = "bold blue"/' ${STARSHIP_CONFIG} && \
    printf "\nexport STARSHIP_CONFIG=${STARSHIP_CONFIG}\neval \"\$(starship init bash)\"\n" >> ~/.bashrc && \
    printf "\nexport STARSHIP_CONFIG=${STARSHIP_CONFIG}\neval \"\$(starship init zsh)\"\n" >> ~/.zshrc


# AgentJail tab launcher: runs AGENTJAIL_TAB_CMD on the first prompt in the tab.
# AGENTJAIL_TAB_CMD is exported by per-tab wrapper scripts written at launch time.
# Using precmd_functions (zsh) / PROMPT_COMMAND (bash) guarantees the hook runs
# after the full rc file is sourced (so mise, starship, and aliases are active).
RUN cat >> ~/.zshrc <<'AGENTJAIL_HOOK'

# AgentJail tab launcher
if [[ -n "${AGENTJAIL_TAB_CMD}" ]]; then
    _agentjail_tab_cmd="${AGENTJAIL_TAB_CMD}"
    unset AGENTJAIL_TAB_CMD
    function _agentjail_launch_once() {
        precmd_functions=(${precmd_functions:#_agentjail_launch_once})
        eval "${_agentjail_tab_cmd}"
    }
    precmd_functions+=(_agentjail_launch_once)
fi
AGENTJAIL_HOOK

RUN cat >> ~/.bashrc <<'AGENTJAIL_HOOK'

# AgentJail tab launcher
if [ -n "${AGENTJAIL_TAB_CMD}" ]; then
    _agentjail_tab_cmd="${AGENTJAIL_TAB_CMD}"
    unset AGENTJAIL_TAB_CMD
    _agentjail_launch_once() {
        PROMPT_COMMAND="${PROMPT_COMMAND//_agentjail_launch_once;/}"
        PROMPT_COMMAND="${PROMPT_COMMAND//_agentjail_launch_once/}"
        eval "${_agentjail_tab_cmd}"
    }
    PROMPT_COMMAND="_agentjail_launch_once;${PROMPT_COMMAND}"
fi
AGENTJAIL_HOOK

ENV HOSTNAME=${HOSTNAME}

# Startup script: downloads the configured file browser into the persistent
# .agentjail/bin volume if it is not already available in the image.
# This runs on every container start but exits immediately when the binary is
# already present (either built into the image or previously cached).
RUN cat > /usr/local/bin/agentjail-install-browser <<'INSTALL_BROWSER'
#!/bin/bash
set -e
browser="${FILE_BROWSER:-rovr}"
agentjail_bin="/root/.agentjail/bin"
mkdir -p "$agentjail_bin"

# Already available — built into the image or cached from a prior run.
if command -v "$browser" > /dev/null 2>&1 || [ -x "$agentjail_bin/$browser" ]; then
    exit 0
fi

echo "agentjail: '$browser' not found in image — downloading..."
case "$browser" in
    yazi)
        ver=$(curl -sf https://api.github.com/repos/sxyazi/yazi/releases/latest | jq -r '.tag_name')
        curl -fL "https://github.com/sxyazi/yazi/releases/download/${ver}/yazi-x86_64-unknown-linux-gnu.zip" \
            -o /tmp/yazi.zip
        unzip -q /tmp/yazi.zip -d /tmp/yazi-dl
        mv "/tmp/yazi-dl/yazi-x86_64-unknown-linux-gnu/yazi" "$agentjail_bin/yazi"
        chmod +x "$agentjail_bin/yazi"
        rm -rf /tmp/yazi.zip /tmp/yazi-dl
        ;;
    nnn)
        apt-get update -y -q
        apt-get install -y -q nnn
        rm -rf /var/lib/apt/lists/*
        cp /usr/bin/nnn "$agentjail_bin/nnn"
        ;;
    spf)
        ver=$(curl -sf https://api.github.com/repos/yorukot/superfile/releases/latest | jq -r '.tag_name')
        curl -fL "https://github.com/yorukot/superfile/releases/download/${ver}/superfile-linux-${ver}-amd64.tar.gz" \
            -o /tmp/spf.tar.gz
        tar -xf /tmp/spf.tar.gz -C /tmp/
        mv /tmp/dist/spf "$agentjail_bin/spf"
        chmod +x "$agentjail_bin/spf"
        rm -rf /tmp/spf.tar.gz /tmp/dist
        ;;
    *)
        echo "agentjail: unknown file browser '$browser' — skipping auto-install." >&2
        ;;
esac
INSTALL_BROWSER
RUN chmod +x /usr/local/bin/agentjail-install-browser

# Set working directory
WORKDIR /project

RUN [ ! -f /project/mise.toml ] && touch /project/mise.toml

# Default command
CMD /usr/local/bin/agentjail-install-browser || true && mise trust --yes && mise install || rich "No mise file detected" --print --style 'dark_goldenrod' && ${SHELL} && cd /project
