{ lib, ... }:

{
  boot.loader.systemd-boot.enable = lib.mkDefault true;
  boot.loader.efi.canTouchEfiVariables = lib.mkDefault true;
  fileSystems."/" = {
    device = lib.mkDefault "/dev/vda1";
    fsType = lib.mkDefault "ext4";
  };
  networking.hostName = "cairn";
  system.stateVersion = "24.11";

  virtualisation.docker.enable = true;

  services.cairn = {
    enable = true;
    openFirewall = true;
    server.host = "0.0.0.0";

    dispatcher.enable = true;
    dispatcher.workers = [
      {
        name = "claudecode_deepseek";
        type = "claudecode";
        taskTypes = [
          "bootstrap"
          "reason"
          "explore"
        ];
        maxRunning = 2;
        env = {
          ANTHROPIC_MODEL = "deepseek-v4-pro";
          ANTHROPIC_BASE_URL = "https://api.deepseek.com/anthropic";
        };
      }
    ];
  };
}
