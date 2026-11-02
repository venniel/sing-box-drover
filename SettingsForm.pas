unit SettingsForm;

interface

uses
  Winapi.Windows, Winapi.Messages, Winapi.OleAcc, System.SysUtils, System.Classes,
  System.Variants, System.Win.ComObj, System.UITypes,
  System.IOUtils, Vcl.Graphics, Vcl.Controls, Vcl.Forms, Vcl.Dialogs, Vcl.StdCtrls,
  Vcl.FileCtrl, Options;

type
  // VCL controls expose no accessible name, and OLEACC only borrows a
  // sibling STATIC's text when the control uses a native Win32 window
  // class. Answer WM_GETOBJECT ourselves and report the label text as the
  // control's accessible name.
  TControlAccessible = class(TInterfacedObject, IAccessible)
  private
    FDefault: IAccessible;
    FName: string;
  public
    constructor Create(const ADefault: IAccessible; const AName: string);
    function Get_accParent(out ppdispParent: IDispatch): HResult; stdcall;
    function Get_accChildCount(out pcountChildren: Integer): HResult; stdcall;
    function Get_accChild(varChild: OleVariant; out ppdispChild: IDispatch): HResult; stdcall;
    function Get_accName(varChild: OleVariant; out pszName: WideString): HResult; stdcall;
    function Get_accValue(varChild: OleVariant; out pszValue: WideString): HResult; stdcall;
    function Get_accDescription(varChild: OleVariant; out pszDescription: WideString): HResult; stdcall;
    function Get_accRole(varChild: OleVariant; out pvarRole: OleVariant): HResult; stdcall;
    function Get_accState(varChild: OleVariant; out pvarState: OleVariant): HResult; stdcall;
    function Get_accHelp(varChild: OleVariant; out pszHelp: WideString): HResult; stdcall;
    function Get_accHelpTopic(out pszHelpFile: WideString; varChild: OleVariant;
      out pidTopic: Integer): HResult; stdcall;
    function Get_accKeyboardShortcut(varChild: OleVariant; out pszKeyboardShortcut: WideString): HResult; stdcall;
    function Get_accFocus(out pvarChild: OleVariant): HResult; stdcall;
    function Get_accSelection(out pvarChildren: OleVariant): HResult; stdcall;
    function Get_accDefaultAction(varChild: OleVariant; out pszDefaultAction: WideString): HResult; stdcall;
    function accSelect(flagsSelect: Integer; varChild: OleVariant): HResult; stdcall;
    function accLocation(out pxLeft: Integer; out pyTop: Integer; out pcxWidth: Integer;
      out pcyHeight: Integer; varChild: OleVariant): HResult; stdcall;
    function accNavigate(navDir: Integer; varStart: OleVariant; out pvarEndUpAt: OleVariant): HResult; stdcall;
    function accHitTest(xLeft: Integer; yTop: Integer; out pvarChild: OleVariant): HResult; stdcall;
    function accDoDefaultAction(varChild: OleVariant): HResult; stdcall;
    function Set_accName(varChild: OleVariant; const pszName: WideString): HResult; stdcall;
    function Set_accValue(varChild: OleVariant; const pszValue: WideString): HResult; stdcall;
    function GetTypeInfoCount(out Count: Integer): HResult; stdcall;
    function GetTypeInfo(Index, LocaleID: Integer; out TypeInfo): HResult; stdcall;
    function GetIDsOfNames(const IID: TGUID; Names: Pointer; NameCount, LocaleID: Integer;
      DispIDs: Pointer): HResult; stdcall;
    function Invoke(DispID: Integer; const IID: TGUID; LocaleID: Integer; Flags: Word;
      var Params; VarResult, ExcepInfo, ArgErr: Pointer): HResult; stdcall;
  end;

  TAccessibleEdit = class(TEdit)
  private
    FAccessible: IAccessible;
    FAccessibleName: string;
  protected
    procedure WndProc(var Message: TMessage); override;
    procedure DestroyWnd; override;
  published
    property AccessibleName: string read FAccessibleName write FAccessibleName;
  end;

  TAccessibleCombo = class(TComboBox)
  private
    FAccessible: IAccessible;
    FAccessibleName: string;
  protected
    procedure WndProc(var Message: TMessage); override;
    procedure DestroyWnd; override;
  published
    property AccessibleName: string read FAccessibleName write FAccessibleName;
  end;

  TfrmSettings = class(TForm)
    lblCoreDir: TLabel;
    edCoreDir: TAccessibleEdit;
    btnBrowseCore: TButton;
    lblConfig: TLabel;
    edConfig: TAccessibleEdit;
    btnBrowseConfig: TButton;
    lblLayout: TLabel;
    cbLayout: TAccessibleCombo;
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

function IsSelfChild(const varChild: OleVariant): Boolean;
begin
  if VarIsNull(varChild) or VarIsEmpty(varChild) then
    exit(true);

  try
    result := Integer(varChild) = CHILDID_SELF;
  except
    result := false;
  end;
end;

constructor TControlAccessible.Create(const ADefault: IAccessible; const AName: string);
begin
  inherited Create;
  FDefault := ADefault;
  FName := AName;
end;

function TControlAccessible.Get_accParent(out ppdispParent: IDispatch): HResult;
begin
  result := FDefault.Get_accParent(ppdispParent);
end;

function TControlAccessible.Get_accChildCount(out pcountChildren: Integer): HResult;
begin
  result := FDefault.Get_accChildCount(pcountChildren);
end;

function TControlAccessible.Get_accChild(varChild: OleVariant; out ppdispChild: IDispatch): HResult;
begin
  result := FDefault.Get_accChild(varChild, ppdispChild);
end;

function TControlAccessible.Get_accName(varChild: OleVariant; out pszName: WideString): HResult;
begin
  if IsSelfChild(varChild) then
  begin
    pszName := FName;
    result := S_OK;
  end
  else
    result := FDefault.Get_accName(varChild, pszName);
end;

function TControlAccessible.Get_accValue(varChild: OleVariant; out pszValue: WideString): HResult;
begin
  result := FDefault.Get_accValue(varChild, pszValue);
end;

function TControlAccessible.Get_accDescription(varChild: OleVariant; out pszDescription: WideString): HResult;
begin
  result := FDefault.Get_accDescription(varChild, pszDescription);
end;

function TControlAccessible.Get_accRole(varChild: OleVariant; out pvarRole: OleVariant): HResult;
begin
  result := FDefault.Get_accRole(varChild, pvarRole);
end;

function TControlAccessible.Get_accState(varChild: OleVariant; out pvarState: OleVariant): HResult;
begin
  result := FDefault.Get_accState(varChild, pvarState);
end;

function TControlAccessible.Get_accHelp(varChild: OleVariant; out pszHelp: WideString): HResult;
begin
  result := FDefault.Get_accHelp(varChild, pszHelp);
end;

function TControlAccessible.Get_accHelpTopic(out pszHelpFile: WideString; varChild: OleVariant;
  out pidTopic: Integer): HResult;
begin
  result := FDefault.Get_accHelpTopic(pszHelpFile, varChild, pidTopic);
end;

function TControlAccessible.Get_accKeyboardShortcut(varChild: OleVariant;
  out pszKeyboardShortcut: WideString): HResult;
begin
  result := FDefault.Get_accKeyboardShortcut(varChild, pszKeyboardShortcut);
end;

function TControlAccessible.Get_accFocus(out pvarChild: OleVariant): HResult;
begin
  result := FDefault.Get_accFocus(pvarChild);
end;

function TControlAccessible.Get_accSelection(out pvarChildren: OleVariant): HResult;
begin
  result := FDefault.Get_accSelection(pvarChildren);
end;

function TControlAccessible.Get_accDefaultAction(varChild: OleVariant;
  out pszDefaultAction: WideString): HResult;
begin
  result := FDefault.Get_accDefaultAction(varChild, pszDefaultAction);
end;

function TControlAccessible.accSelect(flagsSelect: Integer; varChild: OleVariant): HResult;
begin
  result := FDefault.accSelect(flagsSelect, varChild);
end;

function TControlAccessible.accLocation(out pxLeft: Integer; out pyTop: Integer;
  out pcxWidth: Integer; out pcyHeight: Integer; varChild: OleVariant): HResult;
begin
  result := FDefault.accLocation(pxLeft, pyTop, pcxWidth, pcyHeight, varChild);
end;

function TControlAccessible.accNavigate(navDir: Integer; varStart: OleVariant;
  out pvarEndUpAt: OleVariant): HResult;
begin
  result := FDefault.accNavigate(navDir, varStart, pvarEndUpAt);
end;

function TControlAccessible.accHitTest(xLeft: Integer; yTop: Integer;
  out pvarChild: OleVariant): HResult;
begin
  result := FDefault.accHitTest(xLeft, yTop, pvarChild);
end;

function TControlAccessible.accDoDefaultAction(varChild: OleVariant): HResult;
begin
  result := FDefault.accDoDefaultAction(varChild);
end;

function TControlAccessible.Set_accName(varChild: OleVariant; const pszName: WideString): HResult;
begin
  result := FDefault.Set_accName(varChild, pszName);
end;

function TControlAccessible.Set_accValue(varChild: OleVariant; const pszValue: WideString): HResult;
begin
  result := FDefault.Set_accValue(varChild, pszValue);
end;

function TControlAccessible.GetTypeInfoCount(out Count: Integer): HResult;
begin
  result := FDefault.GetTypeInfoCount(Count);
end;

function TControlAccessible.GetTypeInfo(Index, LocaleID: Integer; out TypeInfo): HResult;
begin
  result := FDefault.GetTypeInfo(Index, LocaleID, TypeInfo);
end;

function TControlAccessible.GetIDsOfNames(const IID: TGUID; Names: Pointer;
  NameCount, LocaleID: Integer; DispIDs: Pointer): HResult;
begin
  result := FDefault.GetIDsOfNames(IID, Names, NameCount, LocaleID, DispIDs);
end;

function TControlAccessible.Invoke(DispID: Integer; const IID: TGUID; LocaleID: Integer;
  Flags: Word; var Params; VarResult, ExcepInfo, ArgErr: Pointer): HResult;
begin
  result := FDefault.Invoke(DispID, IID, LocaleID, Flags, Params, VarResult, ExcepInfo, ArgErr);
end;

function CreateControlAccessible(AHandle: HWND; const AClassName, AName: string): IAccessible;
var
  acc: IAccessible;
begin
  if Failed(CreateStdAccessibleProxy(AHandle, PChar(AClassName), OBJID_CLIENT,
    IID_IAccessible, acc)) then
    exit(nil);

  result := TControlAccessible.Create(acc, AName);
end;

procedure TAccessibleEdit.WndProc(var Message: TMessage);
begin
  if (Message.Msg = WM_GETOBJECT) and (Integer(Message.LParam) = Integer(OBJID_CLIENT)) then
  begin
    if FAccessible = nil then
      FAccessible := CreateControlAccessible(Handle, 'Edit', FAccessibleName);

    if FAccessible <> nil then
    begin
      Message.Result := LresultFromObject(IID_IAccessible, Message.WParam, FAccessible);
      exit;
    end;
  end;

  inherited;
end;

procedure TAccessibleEdit.DestroyWnd;
begin
  FAccessible := nil;
  inherited;
end;

procedure TAccessibleCombo.WndProc(var Message: TMessage);
begin
  if (Message.Msg = WM_GETOBJECT) and (Integer(Message.LParam) = Integer(OBJID_CLIENT)) then
  begin
    if FAccessible = nil then
      FAccessible := CreateControlAccessible(Handle, 'ComboBox', FAccessibleName);

    if FAccessible <> nil then
    begin
      Message.Result := LresultFromObject(IID_IAccessible, Message.WParam, FAccessible);
      exit;
    end;
  end;

  inherited;
end;

procedure TAccessibleCombo.DestroyWnd;
begin
  FAccessible := nil;
  inherited;
end;

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
