FROM ubuntu:24.04 as base

ARG SHELL="bash"
ARG EDITOR="micro"
ARG USE_COPILOT="false"
ARG USE_OPENCODE="false"
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

ENV SHELL=${SHELL}

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
    eza \
    fd \
    fzf \
    node \
    ripgrep \
    television \
    yq \
    pipx:rich-cli \
    pipx:rovr

#FROM base as agentjail
#RUN curl -sS https://starship.rs/install.sh | sh -s -- -p unknown-linux-gnu -a x86_64

#rm ~/.config/starship.toml && \

# aliases
RUN echo "alias files='rovr'" >> ~/.${SHELL}rc && \
    echo "alias edit='${EDITOR}'" >> ~/.${SHELL}rc && \
    echo "alias /exit=exit" >> ~/.${SHELL}rc && \
    echo "\n\n" >> ~/.${SHELL}rc

# configure prompt
#RUN starship init ${SHELL} >> ~/.${SHELL}rc && \
#    wget https://github.com/mattmc3/zephyr/raw/refs/heads/main/plugins/prompt/themes/hydro.toml -O ~/.config/starship.toml

# install opencode
RUN [ "${USE_OPENCODE}" = "true" ] && curl -LsSf https://opencode.com/install.sh | sh || echo "Not installing opencode"

# install copilot
RUN [ "${USE_COPILOT}" = "true" ] && curl -fsSL https://gh.io/copilot-install | bash || echo "Not installing copilot"

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
    AVAILABLE_EDITORS="Available Editors: vim, nano, micro (Primary: [bold blue]${EDITOR}[/bold blue])" && \
    ALIASES="Available Aliases: [blue][bold]\"files\"[/bold]: [green]rovr[/green], [bold]\"edit\"[/bold]: [green]${EDITOR}[/green][/blue]" && \
    printf "rich '[b]Welcome to [bold white]AgentJail![/bold white][/b]' --print --rule --rule-style 'bold blue' && rich 'You are now in a container running [bold gold3]$PRETTY_NAME[/bold gold3] \nYour project is mounted at [bold gold3]/project[/bold gold3] \nAvailable Package Managers: [bold dark_goldenrod]$PKG_MANAGER, mise, uv, pip, npm[/bold dark_goldenrod] \nDefault shell: [bold dark_goldenrod]$SHELL[/bold dark_goldenrod] \n \n$AVAILABLE_AGENTS \n \n$AVAILABLE_EDITORS \n \n$ALIASES \n\n type [red bold]exit[/red bold] to exit the container \n\nhave fun, and most importantly, \n[bold medium_orchid]~~ v i b e ~~[/bold medium_orchid]\n\n' --print --style 'blue'" >>  ~/.${SHELL}rc

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


ENV HOSTNAME=${HOSTNAME}

# Set working directory
WORKDIR /project

RUN [ ! -f /project/mise.toml ] && touch /project/mise.toml

# Default command
CMD mise trust && mise install || rich "No mise file detected" --print --style 'dark_goldenrod' && ${SHELL} && cd /project
