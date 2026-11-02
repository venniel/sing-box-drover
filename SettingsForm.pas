unit SettingsForm;

interface

uses
  Winapi.Windows, System.SysUtils, System.Classes, System.UITypes,
  System.IOUtils, Vcl.Graphics, Vcl.Controls, Vcl.Forms, Vcl.Dialogs, Vcl.StdCtrls,
  Vcl.FileCtrl, Options;

type
  TfrmSettings = class(TForm)
    lblCoreDir: TLabel;
    edCoreDir: TEdit;
    btnBrowseCore: TButton;
    lblConfig: TLabel;
    edConfig: TEdit;
    btnBrowseConfig: TButton;
    lblLayout: TLabel;
    cbLayout: TComboBox;
    lblError: TLabel;
    btnSave: TButton;
    btnCancel: TButton;
    procedure FormCreate(Sender: TObject);
  private
    FOptionsFile: string;
    FInitialCoreDir: string;
    FInitialConfigFile: string;
    FRestartRequired: Boolean;
    FSelectedLayout: TSelectorMenuLayout;

    function CleanPath(const AValue: string): string;
    function NormalizeDir(const AValue: string): string;
    procedure ShowError(const AMessage: string);
    procedure Finish(AResult: TModalResult);

    procedure BrowseCoreClick(Sender: TObject);
    procedure BrowseConfigClick(Sender: TObject);
    procedure SaveClick(Sender: TObject);
    procedure CancelClick(Sender: TObject);
  protected
    procedure CreateParams(var Params: TCreateParams); override;
  public
    procedure Init(AFirstRun: Boolean; const AErrorMessage: string);

    property RestartRequired: Boolean read FRestartRequired;
    property SelectedLayout: TSelectorMenuLayout read FSelectedLayout;
  end;

var
  frmSettings: TfrmSettings;

implementation

{$R *.dfm}

procedure TfrmSettings.CreateParams(var Params: TCreateParams);
begin
  inherited;
  Params.ExStyle := Params.ExStyle or WS_EX_APPWINDOW;
end;

procedure TfrmSettings.FormCreate(Sender: TObject);
var
  opts: TDroverOptions;
begin
  FOptionsFile := IncludeTrailingPathDelimiter(ExtractFilePath(ParamStr(0))) + OPTIONS_FILENAME;

  opts := TDroverOptions.Load(FOptionsFile);

  try
    FInitialCoreDir := NormalizeDir(opts.sbDir);
  except
    FInitialCoreDir := opts.sbDir;
  end;

  try
    FInitialConfigFile := TPath.GetFullPath(opts.sbConfigFile);
  except
    FInitialConfigFile := opts.sbConfigFile;
  end;

  edCoreDir.Text := FInitialCoreDir;
  edConfig.Text := opts.sbConfigFile;

  cbLayout.Items.Add('自动');
  cbLayout.Items.Add('扁平');
  cbLayout.Items.Add('嵌套');
  case opts.selectorMenuLayout of
    smlFlat:
      cbLayout.ItemIndex := 1;
    smlNested:
      cbLayout.ItemIndex := 2;
  else
    cbLayout.ItemIndex := 0;
  end;

  btnBrowseCore.OnClick := BrowseCoreClick;
  btnBrowseConfig.OnClick := BrowseConfigClick;
  btnSave.OnClick := SaveClick;
  btnCancel.OnClick := CancelClick;
end;

procedure TfrmSettings.Init(AFirstRun: Boolean; const AErrorMessage: string);
begin
  if AFirstRun then
    Caption := 'sing-box-drover 初始设置'
  else
    Caption := 'sing-box-drover 设置';

  if AErrorMessage <> '' then
    ShowError(AErrorMessage);
end;

function TfrmSettings.CleanPath(const AValue: string): string;
var
  p: string;
begin
  p := AValue.Trim;
  if (Length(p) >= 2) and (p[1] = '"') and (p[Length(p)] = '"') then
    p := Copy(p, 2, Length(p) - 2).Trim;
  result := p;
end;

function TfrmSettings.NormalizeDir(const AValue: string): string;
var
  p: string;
begin
  p := CleanPath(AValue);
  if p = '' then
    exit('');

  p := TPath.GetFullPath(p);
  if (Length(p) > 3) and IsPathDelimiter(p, Length(p)) then
    p := ExcludeTrailingPathDelimiter(p);

  result := p;
end;

procedure TfrmSettings.ShowError(const AMessage: string);
begin
  lblError.Caption := AMessage;
  lblError.Visible := true;
end;

procedure TfrmSettings.Finish(AResult: TModalResult);
begin
  ModalResult := AResult;
  if not (fsModal in FormState) then
    Close;
end;

procedure TfrmSettings.BrowseCoreClick(Sender: TObject);
var
  dir: string;
begin
  dir := CleanPath(edCoreDir.Text);
  if not System.SysUtils.DirectoryExists(dir) then
    dir := '';

  if SelectDirectory('选择 sing-box 核心目录', '', dir, [sdNewUI], Self) then
    edCoreDir.Text := dir;
end;

procedure TfrmSettings.BrowseConfigClick(Sender: TObject);
var
  dlg: TOpenDialog;
  current, coreDir: string;
begin
  dlg := TOpenDialog.Create(Self);
  try
    dlg.Filter := '配置文件 (*.json)|*.json|所有文件 (*.*)|*.*';
    dlg.Options := [ofFileMustExist];

    current := CleanPath(edConfig.Text);
    if FileExists(current) then
    begin
      dlg.InitialDir := ExtractFileDir(current);
      dlg.FileName := ExtractFileName(current);
    end
    else
    begin
      coreDir := CleanPath(edCoreDir.Text);
      if System.SysUtils.DirectoryExists(coreDir) then
        dlg.InitialDir := coreDir;
    end;

    if dlg.Execute(Self.Handle) then
      edConfig.Text := dlg.FileName;
  finally
    dlg.Free;
  end;
end;

procedure TfrmSettings.SaveClick(Sender: TObject);
var
  coreDir, configFile: string;
  layout: TSelectorMenuLayout;
begin
  try
    coreDir := NormalizeDir(edCoreDir.Text);
  except
    ShowError('核心目录路径无效。');
    edCoreDir.SetFocus;
    exit;
  end;

  if coreDir = '' then
  begin
    ShowError('请填写 sing-box 核心目录。');
    edCoreDir.SetFocus;
    exit;
  end;

  if not FileExists(IncludeTrailingPathDelimiter(coreDir) + 'sing-box.exe') then
  begin
    ShowError('在 ' + coreDir + ' 中未找到 sing-box.exe。');
    edCoreDir.SetFocus;
    exit;
  end;

  configFile := CleanPath(edConfig.Text);
  if configFile = '' then
  begin
    ShowError('请填写配置文件路径。');
    edConfig.SetFocus;
    exit;
  end;

  try
    if not TPath.IsPathRooted(configFile) then
      configFile := TPath.Combine(coreDir, configFile);
    configFile := TPath.GetFullPath(configFile);
  except
    ShowError('配置文件路径无效。');
    edConfig.SetFocus;
    exit;
  end;

  if not FileExists(configFile) then
  begin
    ShowError('找不到配置文件 ' + configFile + '。');
    edConfig.SetFocus;
    exit;
  end;

  case cbLayout.ItemIndex of
    1:
      layout := smlFlat;
    2:
      layout := smlNested;
  else
    layout := smlAuto;
  end;

  try
    TDroverOptions.Save(FOptionsFile, coreDir, configFile, layout);
  except
    on E: Exception do
    begin
      ShowError('保存设置失败：' + E.Message);
      exit;
    end;
  end;

  FSelectedLayout := layout;
  FRestartRequired := (not SameText(coreDir, FInitialCoreDir)) or
    (not SameText(configFile, FInitialConfigFile));

  Finish(mrOk);
end;

procedure TfrmSettings.CancelClick(Sender: TObject);
begin
  Finish(mrCancel);
end;

end.
