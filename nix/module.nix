{ config
, lib
, pkgs
, ...
}:

let
  inherit (lib)
    mkEnableOption
    mkIf
    mkOption
    optional
    optionalAttrs
    types
    literalExpression
    ;

  cfg = config.services.cairn;

  workerType = types.enum [
    "claudecode"
    "codex"
    "pi"
    "mock"
  ];
  taskType = types.enum [
    "bootstrap"
    "reason"
    "explore"
  ];

  workerOptions = {
    options = {
      name = mkOption {
        type = types.str;
        description = "Unique worker name; also the identity written back to the protocol.";
      };
      type = mkOption {
        type = workerType;
        description = "Worker backend driver.";
      };
      taskTypes = mkOption {
        type = types.nonEmptyListOf taskType;
        description = "Task types this worker may run.";
      };
      maxRunning = mkOption {
        type = types.ints.positive;
        default = 1;
        description = "Per-worker concurrency cap.";
      };
      priority = mkOption {
        type = types.ints.unsigned;
        default = 0;
        description = "Scheduling priority; lower runs first.";
      };
      env = mkOption {
        type = types.attrsOf types.str;
        default = { };
        description = "Extra environment variables for this worker.";
        example = literalExpression ''
          {
            ANTHROPIC_MODEL = "deepseek-v4-pro";
            ANTHROPIC_BASE_URL = "https://api.deepseek.com/anthropic";
          }
        '';
      };
    };
  };

  workerToAttr = worker: {
    inherit (worker) name type priority;
    task_types = worker.taskTypes;
    max_running = worker.maxRunning;
    env = worker.env;
  };

  dispatcherConfig = {
    server = cfg.dispatcher.serverUrl;
    runtime = {
      interval = cfg.dispatcher.interval;
      max_workers = cfg.dispatcher.maxWorkers;
      max_running_projects = cfg.dispatcher.maxRunningProjects;
      max_project_workers = cfg.dispatcher.maxProjectWorkers;
      healthcheck_timeout = cfg.dispatcher.healthcheckTimeout;
      worker_healthcheck = cfg.dispatcher.workerHealthcheck;
      prompt_group = cfg.dispatcher.promptGroup;
      execution = cfg.dispatcher.execution;
    };
    tasks = {
      bootstrap = {
        inherit (cfg.dispatcher.tasks.bootstrap) timeout conclude_timeout;
      };
      reason = {
        inherit (cfg.dispatcher.tasks.reason) timeout max_intents;
      };
      explore = {
        inherit (cfg.dispatcher.tasks.explore) timeout conclude_timeout;
      };
    };
    common_env = cfg.dispatcher.commonEnv;
    workers = map workerToAttr cfg.dispatcher.workers;
  }
  // optionalAttrs (cfg.dispatcher.execution == "container") {
    container = {
      inherit (cfg.dispatcher.container) image network_mode completed_action cap_add;
      platform = cfg.dispatcher.container.platform;
    };
  }
  // optionalAttrs (cfg.dispatcher.execution == "local") {
    local = {
      workspace_root = cfg.dispatcher.local.workspaceRoot;
      inherit (cfg.dispatcher.local) completed_action;
    };
  };

  dispatcherConfigFile = pkgs.writeText "cairn-dispatch.json" (builtins.toJSON dispatcherConfig);

  serverExec = lib.escapeShellArgs (
    [
      "${cfg.package}/bin/cairn"
      "serve"
      "--host"
      cfg.server.host
      "--port"
      (toString cfg.server.port)
      "--db-path"
      "${cfg.stateDir}/cairn.db"
    ]
    ++ optional (!cfg.server.accessLog) "--no-access-log"
  );

  hardening = {
    NoNewPrivileges = true;
    PrivateTmp = true;
    ProtectClock = true;
    ProtectControlGroups = true;
    ProtectHostname = true;
    ProtectKernelLogs = true;
    ProtectKernelModules = true;
    ProtectKernelTunables = true;
    RestrictNamespaces = true;
    RestrictRealtime = true;
    SystemCallArchitectures = "native";
  };
in
{
  options.services.cairn = {
    enable = mkEnableOption "Cairn";

    package = mkOption {
      type = types.package;
      description = ''
        Cairn package to use. When importing `nixosModules.default` it defaults to this
        flake's package; the raw `nix/../../nix/module.nix` module leaves it required.
      '';
    };

    user = mkOption {
      type = types.str;
      default = "cairn";
      description = "User running the server and dispatcher.";
    };

    group = mkOption {
      type = types.str;
      default = "cairn";
      description = "Group running the server and dispatcher.";
    };

    stateDir = mkOption {
      type = types.path;
      default = "/var/lib/cairn";
      description = "Directory holding the database and, in local mode, project workspaces.";
    };

    environmentFile = mkOption {
      type = types.nullOr types.path;
      default = null;
      description = ''
        Environment file loaded by both units, read as `KEY=value` lines.
        Use it for API keys instead of putting secrets in the Nix store.
      '';
    };

    openFirewall = mkEnableOption "the Cairn API port in the firewall";

    server = {
      enable = mkEnableOption "the Cairn API server" // {
        default = true;
      };

      host = mkOption {
        type = types.str;
        default = "127.0.0.1";
        description = "Address the API server binds to.";
      };

      port = mkOption {
        type = types.port;
        default = 8000;
        description = "Port the API server listens on.";
      };

      accessLog = mkOption {
        type = types.bool;
        default = false;
        description = "Log every HTTP request.";
      };
    };

    dispatcher = {
      enable = mkEnableOption "the Cairn dispatcher";

      serverUrl = mkOption {
        type = types.nullOr types.str;
        default = null;
        defaultText = literalExpression ''"http://127.0.0.1:${toString config.services.cairn.server.port}"'';
        description = "API base URL; defaults to the local server instance.";
      };

      instanceId = mkOption {
        type = types.str;
        default = config.networking.hostName;
        defaultText = literalExpression "config.networking.hostName";
        description = ''
          Identity of this dispatcher. Leases are written as `model@instanceId`, so every
          dispatcher sharing one server needs its own value.
        '';
      };

      execution = mkOption {
        type = types.enum [
          "container"
          "local"
        ];
        default = "container";
        description = ''
          `container` runs one worker container per project through the Docker socket.
          `local` runs the agent CLIs directly on this host and reuses their logins.
        '';
      };

      interval = mkOption {
        type = types.ints.positive;
        default = 3;
        description = "Scheduling tick interval in seconds.";
      };

      maxWorkers = mkOption {
        type = types.ints.positive;
        default = 8;
        description = "Global cap on concurrently running tasks.";
      };

      maxRunningProjects = mkOption {
        type = types.ints.positive;
        default = 3;
        description = "Cap on projects scheduled at the same time.";
      };

      maxProjectWorkers = mkOption {
        type = types.ints.positive;
        default = 4;
        description = "Per-project cap on concurrently running tasks.";
      };

      healthcheckTimeout = mkOption {
        type = types.ints.positive;
        default = 20;
        description = "Timeout for a single worker healthcheck.";
      };

      workerHealthcheck = mkOption {
        type = types.enum [
          "startup_and_task"
          "startup_only"
          "disabled"
        ];
        default = "startup_only";
        description = "When worker healthchecks run.";
      };

      promptGroup = mkOption {
        type = types.str;
        default = "default";
        description = "Bundled prompt set to load (`default` or `mock`).";
      };

      logLevel = mkOption {
        type = types.str;
        default = "INFO";
        description = "Dispatcher log level.";
      };

      commonEnv = mkOption {
        type = types.attrsOf types.str;
        default = { };
        description = "Environment variables merged into every worker.";
      };

      workers = mkOption {
        type = types.listOf (types.submodule workerOptions);
        default = [ ];
        description = ''
          Seed worker models. They are written to the database only while it has no models;
          afterwards the model list edited in the UI is authoritative.
        '';
        example = literalExpression ''
          [
            {
              name = "claudecode_deepseek";
              type = "claudecode";
              taskTypes = [ "bootstrap" "reason" "explore" ];
              maxRunning = 2;
              env = {
                ANTHROPIC_MODEL = "deepseek-v4-pro";
                ANTHROPIC_BASE_URL = "https://api.deepseek.com/anthropic";
                ANTHROPIC_AUTH_TOKEN = "sk-...";
              };
            }
          ]
        '';
      };

      extraPackages = mkOption {
        type = types.listOf types.package;
        default = [ ];
        description = "Packages put on the dispatcher PATH; add agent CLIs in local mode.";
      };

      tasks = {
        bootstrap = {
          timeout = mkOption {
            type = types.ints.positive;
            default = 300;
            description = "Bootstrap timeout in seconds.";
          };
          conclude_timeout = mkOption {
            type = types.ints.positive;
            default = 90;
            description = "Bootstrap conclude fallback timeout in seconds.";
          };
        };
        reason = {
          timeout = mkOption {
            type = types.ints.positive;
            default = 300;
            description = "Reason timeout in seconds.";
          };
          max_intents = mkOption {
            type = types.ints.positive;
            default = 2;
            description = "Cap on intents one reason step may create.";
          };
        };
        explore = {
          timeout = mkOption {
            type = types.ints.positive;
            default = 300;
            description = "Explore timeout in seconds.";
          };
          conclude_timeout = mkOption {
            type = types.ints.positive;
            default = 90;
            description = "Explore conclude fallback timeout in seconds.";
          };
        };
      };

      container = {
        image = mkOption {
          type = types.str;
          default = "ghcr.io/oritera/cairn-worker-container:latest";
          description = "Worker container image.";
        };
        network_mode = mkOption {
          type = types.str;
          default = "host";
          description = "Docker network mode for worker containers.";
        };
        completed_action = mkOption {
          type = types.enum [
            "stop"
            "remove"
          ];
          default = "stop";
          description = "What to do with a worker container after a run.";
        };
        platform = mkOption {
          type = types.nullOr types.str;
          default = null;
          description = ''
            Optional Docker platform override, e.g. `"linux/amd64"` to pin an architecture
            under emulation. Left unset, containers use the host architecture and a
            multi-arch image resolves to its matching variant.
          '';
        };
        cap_add = mkOption {
          type = types.listOf types.str;
          default = [ ];
          description = "Extra Linux capabilities for worker containers.";
          example = [
            "NET_RAW"
            "NET_ADMIN"
          ];
        };
      };

      local = {
        workspaceRoot = mkOption {
          type = types.nullOr types.path;
          default = null;
          defaultText = literalExpression "''${config.services.cairn.stateDir}/workspaces";
          description = "Root of per-project worker working directories.";
        };
        completed_action = mkOption {
          type = types.enum [
            "keep"
            "remove"
          ];
          default = "keep";
          description = "What to do with a working directory after a run.";
        };
      };
    };
  };

  config = mkIf cfg.enable (
    let
      serverUrl = if cfg.dispatcher.serverUrl != null then cfg.dispatcher.serverUrl else "http://127.0.0.1:${toString cfg.server.port}";
      workspaceRoot = if cfg.dispatcher.local.workspaceRoot != null then cfg.dispatcher.local.workspaceRoot else "${cfg.stateDir}/workspaces";
      dispatcherConfig' = dispatcherConfig // {
        server = serverUrl;
      };
      dispatcherConfigFile' = pkgs.writeText "cairn-dispatch.json" (builtins.toJSON dispatcherConfig');
      useDocker = cfg.dispatcher.enable && cfg.dispatcher.execution == "container";
    in
    {
      assertions = [
        {
          assertion = cfg.server.enable || !cfg.dispatcher.enable || cfg.dispatcher.serverUrl != null;
          message = "services.cairn.dispatcher.serverUrl must be set when the local server is disabled.";
        }
        {
          assertion = cfg.dispatcher.maxProjectWorkers <= cfg.dispatcher.maxWorkers;
          message = "services.cairn.dispatcher.maxProjectWorkers cannot exceed maxWorkers.";
        }
        {
          assertion = !cfg.dispatcher.enable || cfg.dispatcher.execution != "container" || cfg.dispatcher.container.image != "";
          message = "services.cairn.dispatcher.container.image must not be empty in container mode.";
        }
      ];

      warnings = optional (cfg.dispatcher.enable && cfg.dispatcher.workers == [ ]) ''
        services.cairn.dispatcher.workers is empty. The dispatcher will idle until models exist:
        either add seed workers here or create them on the Models page.
      '';

      users.users = optionalAttrs (cfg.user == "cairn") {
        cairn = {
          isSystemUser = true;
          inherit (cfg) group;
          home = cfg.stateDir;
          description = "Cairn service user";
        };
      };
      users.groups = optionalAttrs (cfg.group == "cairn") { cairn = { }; };

      systemd.tmpfiles.rules = [
        "d ${cfg.stateDir} 0750 ${cfg.user} ${cfg.group} -"
      ]
      ++ optional cfg.dispatcher.enable "d ${workspaceRoot} 0750 ${cfg.user} ${cfg.group} -";

      networking.firewall.allowedTCPPorts = optional (cfg.openFirewall && cfg.server.enable) cfg.server.port;

      systemd.services.cairn-server = mkIf cfg.server.enable {
        description = "Cairn API server";
        wantedBy = [ "multi-user.target" ];
        wants = [ "network-online.target" ];
        after = [ "network-online.target" ];

        environment = {
          HOME = cfg.stateDir;
        }
        // optionalAttrs (cfg.environmentFile != null) {
          EnvironmentFile = cfg.environmentFile;
        };

        serviceConfig = hardening // {
          Type = "simple";
          User = cfg.user;
          Group = cfg.group;
          WorkingDirectory = cfg.stateDir;
          ExecStart = serverExec;
          Restart = "on-failure";
          RestartSec = 5;
          ReadWritePaths = [ cfg.stateDir ];
          ProtectSystem = "strict";
          ProtectHome = true;
          PrivateDevices = true;
          RestrictAddressFamilies = [
            "AF_INET"
            "AF_INET6"
            "AF_UNIX"
          ];
        };
      };

      systemd.services.cairn-dispatcher = mkIf cfg.dispatcher.enable {
        description = "Cairn dispatcher";
        wantedBy = [ "multi-user.target" ];
        wants = [ "network-online.target" ] ++ optional cfg.server.enable "cairn-server.service";
        after = [ "network-online.target" ] ++ optional cfg.server.enable "cairn-server.service";
        requires = optional (useDocker && config.virtualisation.docker.enable) "docker.service";

        path = cfg.dispatcher.extraPackages;
        environment = {
          HOME = cfg.stateDir;
          CAIRN_INSTANCE_ID = cfg.dispatcher.instanceId;
        };

        serviceConfig = hardening // {
          Type = "simple";
          User = cfg.user;
          Group = cfg.group;
          SupplementaryGroups = optional useDocker "docker";
          WorkingDirectory = cfg.stateDir;
          ExecStart = lib.escapeShellArgs [
            "${cfg.package}/bin/cairn"
            "dispatch"
            "--config"
            dispatcherConfigFile'
            "--log-level"
            cfg.dispatcher.logLevel
          ];
          Restart = "on-failure";
          RestartSec = 10;
          ReadWritePaths = [ cfg.stateDir ] ++ optional (cfg.dispatcher.execution == "local") workspaceRoot;
          PrivateDevices = cfg.dispatcher.execution == "local";
          RestrictAddressFamilies = [
            "AF_INET"
            "AF_INET6"
            "AF_UNIX"
            "AF_NETLINK"
          ];
        }
          // optionalAttrs useDocker {
          ProtectSystem = "strict";
          ProtectHome = true;
        }
          // optionalAttrs (cfg.environmentFile != null) {
          EnvironmentFile = cfg.environmentFile;
        };
      };
    }
  );
}
