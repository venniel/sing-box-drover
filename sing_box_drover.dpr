program sing_box_drover;

{$R *.dres}

uses
  Winapi.Windows,
  Winapi.Messages,
  System.SysUtils,
  System.UITypes,
  Vcl.Forms,
  Main in 'Main.pas' {frmMain},
  SettingsForm in 'SettingsForm.pas' {frmSettings},
  SystemProxy in 'SystemProxy.pas',
  Drover in 'Drover.pas',
  Options in 'Options.pas',
  JsonUtils in 'JsonUtils.pas',
  CoreSupervisor in 'CoreSupervisor.pas',
  Logger in 'Logger.pas',
  AppArgs in 'AppArgs.pas',
  AppElevation in 'AppElevation.pas',
  AppSingleInstance in 'AppSingleInstance.pas',
  SingBoxConfig in 'SingBoxConfig.pas',
  CoreApiClient in 'CoreApiClient.pas',
  SingBoxBpf in 'SingBoxBpf.pas',
  ConfigReader in 'ConfigReader.pas',
  ConfigUpdater in 'ConfigUpdater.pas',
  ElevatedTrayIcon in 'ElevatedTrayIcon.pas',
  Autostart in 'Autostart.pas',
  SingBoxCli in 'SingBoxCli.pas',
  AppState in 'AppState.pas',
  TunCleanup in 'TunCleanup.pas';

{$R *.res}

const
  APP_TITLE = 'sing-box-drover';

var
  Drover: TDrover;
  flags: TAppFlags;
  s: string;
  firstRun: boolean;

function ShowSettingsDialog(AFirstRun: boolean; const AError: string): TModalResult;
var
  dlg: TfrmSettings;
begin
  dlg := TfrmSettings.Create(nil);
  try
    dlg.Init(AFirstRun, AError);
    result := dlg.ShowModal;
  finally
    dlg.Free;
  end;
end;

begin
  Application.Initialize;
  Application.Title := APP_TITLE;
  Application.MainFormOnTaskbar := false;
  Application.ShowMainForm := false;

  Drover := nil;
  try
    flags := GetAppFlags;

    if not AcquireSingleInstance('SingBoxDrover_SingleInstance_Mutex', afRestart in flags) then
      raise Exception.Create('Another instance of this application is already running.');

    firstRun := not FileExists(IncludeTrailingPathDelimiter(ExtractFilePath(ParamStr(0))) +
      OPTIONS_FILENAME);

    while true do
    begin
      if firstRun then
      begin
        if ShowSettingsDialog(true, '') <> mrOk then
          exit;
        firstRun := false;
      end;

      try
        Drover := TDrover.Create(flags);
        break;
      except
        on E: Exception do
        begin
          if ShowSettingsDialog(false, E.Message) <> mrOk then
          begin
            MessageBox(0, PChar(E.Message), PChar(APP_TITLE), MB_ICONERROR);
            exit;
          end;
        end;
      end;
    end;

    if Drover.NeedsElevation then
    begin
      if LaunchSelf(FlagsToCmdLine([afTun, afRestart]), 0) then
      begin
        FreeAndNil(Drover);
        exit;
      end;
    end;
  except
    on E: Exception do
    begin
      s := E.Message;
      if s = '' then
        s := 'Unknown error.';

      MessageBox(0, PChar(s), PChar(APP_TITLE), MB_ICONERROR);
      exit;
    end;
  end;

  Application.CreateForm(TfrmMain, frmMain);
  frmMain.InitDrover(Drover);
  try
    Application.Run;
  finally
    FreeAndNil(Drover);
  end;

end.
