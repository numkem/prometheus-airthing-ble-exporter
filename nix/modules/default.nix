{
  config,
  lib,
  pkgs,
  ...
}:

with lib;
let
  cfg = config.services.prometheus-airthing-ble-exporter;
in
{
  options.services.prometheus-airthing-ble-exporter = {
    enable = mkEnableOption "Enable the prometheus Airthing BLE exporter";

    package = mkOption {
      type = types.package;
      default = pkgs.prometheus-airthing-ble-exporter;
      description = mdDoc ''
        Package to use for the systemd service.
      '';
    };

    waveSerialNumber = mkOption {
      type = types.ints.unsigned;
      default = 0;
      description = mdDoc ''
        The serial number of the Airthing Wave to probe for data
      '';
    };

    listenAddress = mkOption {
      type = types.str;
      default = ":9456";
      description = mdDoc ''
        Listening address for the exporter. In the <ip>:<port> format.

        The IP can be ommited for 0.0.0.0
      '';
    };

    collectionDuration = mkOption {
      type = types.str;
      default = "30s";
      description = mdDoc ''
        Go duration for how often to prope the Wave for it's data
      '';
    };
  };

  config = mkIf cfg.enable {
    systemd.services.prometheus-airthing-ble-exporter = {
      description = "Airthing Wave BLE exporter for prometheus";
      restartIfChanged = true;

      serviceConfig.ExecStart = "${cfg.package}/bin/prometheus-airthing-ble-exporter -serial ${cfg.waveSerialNumber} -address ${cfg.listenAddress} -collection ${cfg.collectionDuration}";

      ProtectHostname = true;
      PrivateTmp = !config.boot.isContainer;
      PrivateUsers = true;
    };
  };
}
