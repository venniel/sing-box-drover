unit SingBoxConfig;

interface

uses
  SingBoxBpf;

type
  TConfigSourceFormat = (csfJson, csfBpf);

  TConfigSelector = record
    name: string;
    outbounds: TArray<string>;
    defaultIndex: integer;
    defaultName: string;
  end;

  TConfigSelectors = TArray<TConfigSelector>;

  TClashApiConfig = record
    externalController: string;
    externalUi: string;
    secret: string;

    function IsConfigured: boolean;
    function DashboardUrl: string;
  end;

  TConfigSource = record
    filePath: string;
    format: TConfigSourceFormat;
    jsonText: string;
    bpfProfile: TBpfProfile;

    function isBpf: boolean;
  end;

  TSingBoxConfig = record
    clashApi: TClashApiConfig;
    selectors: TConfigSelectors;
    proxyHost: string;
    proxyPort: integer;
    hasTunInbound: boolean;
    jsonWithTun: string;
    jsonWithoutTun: string;
  end;

implementation

uses
  System.SysUtils,
  System.StrUtils,
  System.NetEncoding;

function TConfigSource.isBpf: boolean;
begin
  result := format = csfBpf;
end;

function TClashApiConfig.IsConfigured: boolean;
begin
  result := externalController <> '';
end;

function TClashApiConfig.DashboardUrl: string;
var
  host, base: string;
begin
  result := '';
  if externalController = '' then
    exit;

  // Allow a full dashboard URL to be supplied instead of a folder name.
  if StartsText('http://', externalUi) or StartsText('https://', externalUi) then
    exit(Trim(externalUi));

  host := externalController;
  if Copy(host, 1, 8) = '0.0.0.0:' then
    host := '127.0.0.1' + Copy(host, 8, MaxInt)
  else if host = '0.0.0.0' then
    host := '127.0.0.1';

  base := 'http://' + host + '/ui/';
  if secret <> '' then
    base := base + '?secret=' + TNetEncoding.URL.Encode(secret);

  result := base;
end;

end.
