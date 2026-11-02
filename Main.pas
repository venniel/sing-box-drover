unit Main;

interface

uses
  Winapi.Windows, Winapi.Messages, System.SysUtils, System.Variants, System.Classes, System.UITypes,
  Vcl.Graphics, Vcl.Controls, Vcl.Forms, Vcl.Dialogs, Vcl.ExtCtrls, Vcl.Menus, System.Net.HttpClient,
  System.Net.URLClient, System.JSON, System.IOUtils, System.Generics.Collections, Options, Drover,
  AppElevation, AppArgs, SingBoxConfig, ElevatedTrayIcon, Autostart, Winapi.ShellAPI, CoreSupervisor,
  SettingsForm;

type
  TPendingSelectorRequests = TDictionary<NativeInt, TMenuItem>;

  TfrmMain = class(TForm)
    PopupMenu: TPopupMenu;
    miExtras: TMenuItem;
    miRestart: TMenuItem;
    miAutostart: TMenuItem;
    miHomepage: TMenuItem;
    miModes: TMenuItem;
    miNodes: TMenuItem;
    miTun: TMenuItem;
    miSystemProxy: TMenuItem;
    miDashboard: TMenuItem;
    miQuit: TMenuItem;
    procedure FormCloseQuery(Sender: TObject; var CanClose: boolean);
    procedure miQuitClick(Sender: TObject);
    procedure miSystemProxyClick(Sender: TObject);
    procedure TrayIconClick(Sender: TObject);
    procedure miSelectorClick(Sender: TObject);
    procedure miModeClick(Sender: TObject);
    procedure DrawSelectors;
    procedure DrawModes;
    procedure FormCreate(Sender: TObject);
    procedure miTunClick(Sender: TObject);
    procedure miAutostartClick(Sender: TObject);
    procedure miRestartClick(Sender: TObject);
    procedure miHomepageClick(Sender: TObject);
    procedure miDashboardClick(Sender: TObject);
  private
    TrayIcon: TElevatedTrayIcon;
    FDrover: TDrover;
    FSettingsForm: TfrmSettings;
    FIsTunActive: boolean;
    FIsSystemProxyActive: boolean;
    FClosePending: boolean;
    FSettingsOpening: boolean;
    FSettingsClosed: boolean;
    FSettingsClosedTick: cardinal;
    FLastRequestId: NativeInt;
    FPendingSelectorRequests: TPendingSelectorRequests;

    function NewRequestId: NativeInt;
    procedure UpdateTrayIcon;
    procedure ToggleSystemProxy(AEnable: boolean; AUpdateTrayIcon: boolean = true; APersist: boolean = true);
    procedure ToggleTunDisplay(AActive: boolean; AUpdateTrayIcon: boolean = true);
    procedure ToggleTun(AEnable: boolean);
    procedure HandleDroverEvent(event: TDroverEvent);
    procedure WMDroverCanClose(var msg: TMessage); message WM_DROVER_CAN_CLOSE;
    procedure WMEndSession(var msg: TMessage); message WM_ENDSESSION;
    procedure WMAutostartResult(var msg: TMessage); message WM_AUTOSTART_RESULT;
    procedure ShowBalloon(AText, ATitle: string; AFlags: TBalloonFlags = bfInfo; ATimeout: integer = 10000);
    procedure ShowOnlyExitInTray;
    procedure ShowSettings;
    procedure HandleSettingsClose(Sender: TObject; var Action: TCloseAction);
    procedure InitAutostart;
    procedure StartAutostartThread(AAction: TAutostartAction);
    procedure HandleAutostartResult(const r: TAutostartResult);
  public

    destructor Destroy; override;
    procedure InitDrover(ADrover: TDrover);
  end;

var
  frmMain: TfrmMain;

implementation

{$R *.dfm}

procedure TfrmMain.FormCreate(Sender: TObject);
begin
  FIsTunActive := false;
  FIsSystemProxyActive := false;
  FClosePending := false;
  FSettingsForm := nil;
  FSettingsOpening := false;
  FSettingsClosed := false;
  FSettingsClosedTick := 0;
  FLastRequestId := 0;
  FPendingSelectorRequests := TPendingSelectorRequests.Create;

  miTun.Enabled := false;
  miTun.Visible := false;

  TrayIcon := TElevatedTrayIcon.Create(self);
  TrayIcon.PopupMenu := PopupMenu;
  TrayIcon.OnClick := TrayIconClick;

  InitAutostart;
end;

destructor TfrmMain.Destroy;
begin
  FreeAndNil(FPendingSelectorRequests);
  inherited;
end;

function TfrmMain.NewRequestId: NativeInt;
begin
  inc(FLastRequestId);
  result := FLastRequestId;
end;

procedure TfrmMain.InitAutostart;
var
  flags: TAppFlags;
  initialAction: TAutostartAction;
begin
  miAutostart.Enabled := false;
  miAutostart.Checked := false;

  flags := GetAppFlags;
  if afAutostartEnable in flags then
    initialAction := aaEnable
  else if afAutostartDisable in flags then
    initialAction := aaDisable
  else
    initialAction := aaCheck;

  StartAutostartThread(initialAction);
end;

procedure TfrmMain.StartAutostartThread(AAction: TAutostartAction);
begin
  miAutostart.Enabled := false;
  TAutostartThread.Create(AAction, Handle);
end;

procedure TfrmMain.WMAutostartResult(var msg: TMessage);
var
  p: PAutostartResult;
begin
  p := PAutostartResult(msg.LParam);
  if p = nil then
    exit;
  try
    HandleAutostartResult(p^);
  finally
    Dispose(p);
  end;
end;

procedure TfrmMain.HandleAutostartResult(const r: TAutostartResult);
begin
  miAutostart.Enabled := r.state <> asUnknown;
  miAutostart.Checked := r.state = asEnabled;

  if (not r.success) and (r.action <> aaCheck) then
    ShowBalloon(r.errorMsg, '', bfError);
end;

procedure TfrmMain.miAutostartClick(Sender: TObject);
var
  flag: TAppFlag;
  action: TAutostartAction;
begin
  if FClosePending then
    exit;

  if miAutostart.Checked then
  begin
    flag := afAutostartDisable;
    action := aaDisable;
  end
  else
  begin
    flag := afAutostartEnable;
    action := aaEnable;
  end;

  if not IsProcessElevated then
  begin
    FDrover.PersistRuntimeState;
    if LaunchSelf(FlagsToCmdLine([flag, afRestart]), Handle) then
      Close;
    exit;
  end;

  StartAutostartThread(action);
end;

procedure TfrmMain.miHomepageClick(Sender: TObject);
begin
  ShellExecute(0, 'open', 'https://github.com/hdrover/sing-box-drover', nil, nil, SW_SHOWNORMAL);
end;

procedure TfrmMain.miRestartClick(Sender: TObject);
var
  flags: TAppFlags;
begin
  if FClosePending then
    exit;

  flags := [afRestart];
  if FIsTunActive then
    Include(flags, afTun);

  FDrover.PersistRuntimeState;
  if LaunchSelf(FlagsToCmdLine(flags), Handle, IsProcessElevated) then
    Close;
end;

procedure TfrmMain.miDashboardClick(Sender: TObject);
var
  url: string;
begin
  url := FDrover.sbConfig.clashApi.DashboardUrl;
  if url = '' then
    exit;

  ShellExecute(0, 'open', PChar(url), nil, nil, SW_SHOWNORMAL);
end;

procedure TfrmMain.InitDrover(ADrover: TDrover);
var
  systemProxyEnabled: boolean;
begin
  FDrover := ADrover;
  FDrover.NotifyHandle := Handle;

  DrawSelectors;
  DrawModes;

  miDashboard.Visible := FDrover.sbConfig.clashApi.DashboardUrl <> '';
  miTun.Visible := FDrover.sbConfig.hasTunInbound;
  ToggleTunDisplay(FDrover.IsTunActive, false);

  if not FDrover.StoredSystemProxy(systemProxyEnabled) then
  begin
    systemProxyEnabled := FDrover.Options.systemProxyAuto;
    FDrover.StoreSystemProxy(systemProxyEnabled);
  end;

  if systemProxyEnabled then
    ToggleSystemProxy(true, false, false);

  UpdateTrayIcon;

  TrayIcon.Visible := true;

  FDrover.OnEvent := HandleDroverEvent;
end;

procedure TfrmMain.HandleDroverEvent(event: TDroverEvent);
var
  pendingItem: TMenuItem;
  coreEvent: TCoreEvent;
begin
  case event.kind of
    dekError:
      begin
        ShowBalloon(event.msg, '', bfError);
        DrawModes;
      end;

    dekRunning:
      begin
        ToggleTunDisplay(FDrover.IsTunActive);
        miTun.Enabled := true;
      end;

    dekClashModes:
      DrawModes;

    dekCoreEvent:
      begin
        coreEvent := event.coreEvent;
        case coreEvent.kind of
          cekSelectorDone:
            if FPendingSelectorRequests.TryGetValue(coreEvent.requestId, pendingItem) then
            begin
              FPendingSelectorRequests.Remove(coreEvent.requestId);
              pendingItem.Enabled := true;
            end;
        end;
      end;
  end;
end;

procedure TfrmMain.ShowBalloon(AText, ATitle: string; AFlags: TBalloonFlags = bfInfo; ATimeout: integer = 10000);
begin
  if (AFlags = bfError) and (ATitle = '') then
    ATitle := 'Error';

  TrayIcon.BalloonHint := AText;
  TrayIcon.BalloonTitle := ATitle;
  TrayIcon.BalloonFlags := AFlags;
  TrayIcon.BalloonTimeout := ATimeout;
  TrayIcon.ShowBalloonHint;
end;

procedure TfrmMain.DrawSelectors;
const
  AUTO_FLAT_ROWS_THRESHOLD = 20;
var
  selectors: TConfigSelectors;
  selector: TConfigSelector;
  outboundName: string;
  selectorItem, outboundItem: TMenuItem;
  selectorCount, selectorI, outboundI: integer;
  flatRows: integer;
  isNested: boolean;
  itemOwner: TComponent;
begin
  selectors := FDrover.selectors;
  selectorCount := Length(selectors);

  while miNodes.Count > 0 do
    miNodes.Items[0].Free;

  miNodes.Visible := (selectorCount >= 1) and (selectorCount <= 50);
  if not miNodes.Visible then
    exit;

  case FDrover.Options.selectorMenuLayout of
    smlFlat:
      isNested := false;
    smlNested:
      isNested := true;
  else
    flatRows := 2 * selectorCount;
    for selectorI := Low(selectors) to High(selectors) do
      inc(flatRows, Length(selectors[selectorI].outbounds));
    isNested := flatRows > AUTO_FLAT_ROWS_THRESHOLD;
  end;

  for selectorI := Low(selectors) to High(selectors) do
  begin
    selector := selectors[selectorI];

    selectorItem := TMenuItem.Create(miNodes);
    selectorItem.Caption := selector.name;

    if isNested then
      itemOwner := selectorItem
    else
    begin
      selectorItem.Enabled := false;
      itemOwner := miNodes;
      miNodes.Add(selectorItem);
    end;

    outboundItem := nil;
    for outboundI := Low(selector.outbounds) to High(selector.outbounds) do
    begin
      outboundName := selector.outbounds[outboundI];

      outboundItem := TMenuItem.Create(itemOwner);
      outboundItem.Caption := outboundName;
      outboundItem.AutoCheck := true;
      outboundItem.RadioItem := true;
      outboundItem.OnClick := miSelectorClick;
      outboundItem.Tag := selectorI * 1000 + outboundI;
      outboundItem.Checked := (outboundI = selector.defaultIndex);
      outboundItem.GroupIndex := selectorI + 10;

      if isNested then
        selectorItem.Add(outboundItem)
      else
        miNodes.Add(outboundItem);
    end;

    if isNested then
      miNodes.Add(selectorItem)
    else if Assigned(outboundItem) and (selectorI < High(selectors)) then
      miNodes.InsertNewLineAfter(outboundItem);
  end;
end;

procedure TfrmMain.DrawModes;
var
  modes: TArray<string>;
  i: integer;
  item: TMenuItem;
begin
  modes := FDrover.ClashModes;

  miModes.Visible := Length(modes) > 0;
  if not miModes.Visible then
    exit;

  while miModes.Count > 0 do
    miModes.Delete(0);

  for i := Low(modes) to High(modes) do
  begin
    item := TMenuItem.Create(miModes);
    item.Caption := modes[i];
    item.AutoCheck := true;
    item.RadioItem := true;
    item.GroupIndex := 100;
    item.Checked := SameText(modes[i], FDrover.ClashMode);
    item.OnClick := miModeClick;
    item.Tag := i;
    miModes.Add(item);
  end;
end;

procedure TfrmMain.UpdateTrayIcon;
var
  s: string;
begin
  if FIsTunActive then
  begin
    s := 'TRAY_ICON_TUN';
  end
  else
  begin
    if FIsSystemProxyActive then
      s := 'TRAY_ICON'
    else
      s := 'TRAY_ICON_DISABLED';
  end;

  TrayIcon.Icon.LoadFromResourceName(HInstance, s);
  TrayIcon.Icon := TrayIcon.Icon;
end;

procedure TfrmMain.ToggleSystemProxy(AEnable: boolean; AUpdateTrayIcon: boolean = true; APersist: boolean = true);
begin
  if AEnable and FClosePending then
    exit;

  FIsSystemProxyActive := AEnable;
  miSystemProxy.Checked := AEnable;

  if AEnable then
    FDrover.EnableSystemProxy
  else
    FDrover.DisableSystemProxy;

  if APersist then
    FDrover.StoreSystemProxy(AEnable);

  if AUpdateTrayIcon then
    UpdateTrayIcon;
end;

procedure TfrmMain.ToggleTunDisplay(AActive: boolean; AUpdateTrayIcon: boolean = true);
begin
  FIsTunActive := AActive;
  miTun.Checked := AActive;

  if AUpdateTrayIcon then
    UpdateTrayIcon;
end;

procedure TfrmMain.ToggleTun(AEnable: boolean);
begin
  if AEnable and FClosePending then
    exit;

  if not IsProcessElevated then
  begin
    FDrover.StoreTun(AEnable);
    FDrover.PersistRuntimeState;
    if LaunchSelf(FlagsToCmdLine([afTun, afRestart]), Handle) then
      Close;
    exit;
  end;

  ToggleTunDisplay(AEnable);
  miTun.Enabled := false;
  FDrover.StoreTun(AEnable);
  FDrover.StartCore(AEnable);
end;

procedure TfrmMain.FormCloseQuery(Sender: TObject; var CanClose: boolean);
begin
  if not Assigned(FDrover) then
  begin
    CanClose := true;
    exit;
  end;

  FClosePending := true;
  ShowOnlyExitInTray;

  if Assigned(FSettingsForm) then
    FSettingsForm.Close;

  ToggleTunDisplay(false, false);

  if FIsSystemProxyActive then
    ToggleSystemProxy(false, false, false);

  UpdateTrayIcon;

  CanClose := FDrover.Shutdown;
end;

procedure TfrmMain.WMDroverCanClose(var msg: TMessage);
begin
  if not FClosePending then
    exit;

  EndMenu;
  PostMessage(Handle, WM_CLOSE, 0, 0);
end;

procedure TfrmMain.WMEndSession(var msg: TMessage);
begin
  if msg.WParam = 0 then
    exit;

  FClosePending := true;
  ShowOnlyExitInTray;

  if Assigned(FSettingsForm) then
    FSettingsForm.Close;

  ToggleTunDisplay(false, false);

  if FIsSystemProxyActive then
    ToggleSystemProxy(false, false, false);

  if Assigned(FDrover) then
    FDrover.StopForShutdown;

  UpdateTrayIcon;
end;

procedure TfrmMain.miQuitClick(Sender: TObject);
begin
  Close;
end;

procedure TfrmMain.miSystemProxyClick(Sender: TObject);
begin
  ToggleSystemProxy(not miSystemProxy.Checked);
end;

procedure TfrmMain.miTunClick(Sender: TObject);
begin
  ToggleTun(not miTun.Checked);
end;

procedure TfrmMain.TrayIconClick(Sender: TObject);
begin
  ShowSettings;
end;

procedure TfrmMain.ShowSettings;
var
  settings: TfrmSettings;
begin
  if (not Assigned(FDrover)) or FClosePending then
    exit;

  // A tray activation can arrive just after the keyboard accept (Enter)
  // saved and closed the settings window; don't reopen immediately.
  if FSettingsClosed and (GetTickCount - FSettingsClosedTick < 400) then
    exit;

  if Assigned(FSettingsForm) then
  begin
    FSettingsForm.Show;
    FSettingsForm.BringToFront;
    SetForegroundWindow(FSettingsForm.Handle);
    exit;
  end;

  // Showing the dialog pumps messages, so a second tray activation can
  // re-enter here before FSettingsForm is assigned.
  if FSettingsOpening then
    exit;

  FSettingsOpening := true;
  try
    settings := TfrmSettings.Create(Self);
    settings.Init(false, '');
    settings.OnClose := HandleSettingsClose;
    FSettingsForm := settings;
    settings.Show;
    SetForegroundWindow(settings.Handle);
  finally
    FSettingsOpening := false;
  end;
end;

procedure TfrmMain.HandleSettingsClose(Sender: TObject; var Action: TCloseAction);
var
  settings: TfrmSettings;
begin
  settings := TfrmSettings(Sender);

  Action := caFree;
  settings.OnClose := nil;
  FSettingsForm := nil;
  FSettingsClosed := true;
  FSettingsClosedTick := GetTickCount;

  if FClosePending then
    exit;

  if (settings.ModalResult <> mrOk) or (not Assigned(FDrover)) then
    exit;

  if settings.RestartRequired then
  begin
    miRestartClick(nil);
    exit;
  end;

  FDrover.SetSelectorMenuLayout(settings.SelectedLayout);
  DrawSelectors;
end;

procedure TfrmMain.miSelectorClick(Sender: TObject);
var
  item: TMenuItem;
  i: integer;
  selectorI, outboundI: integer;
  requestId: NativeInt;
begin
  if not(Sender is TMenuItem) then
    exit;

  item := TMenuItem(Sender);
  i := item.Tag;

  selectorI := i div 1000;
  outboundI := i mod 1000;

  item.Checked := true;

  requestId := NewRequestId;

  if not FDrover.EditSelector(selectorI, outboundI, requestId) then
    exit;

  FPendingSelectorRequests.Add(requestId, item);
  item.Enabled := false;
end;

procedure TfrmMain.miModeClick(Sender: TObject);
var
  item: TMenuItem;
begin
  if not(Sender is TMenuItem) then
    exit;

  item := TMenuItem(Sender);
  if SameText(item.Caption, FDrover.ClashMode) then
    exit;

  item.Checked := true;

  if not FDrover.SetClashMode(item.Caption) then
    DrawModes;
end;

procedure TfrmMain.ShowOnlyExitInTray;
begin
  for var item in PopupMenu.Items do
    item.Visible := (item = miQuit);
end;

end.
