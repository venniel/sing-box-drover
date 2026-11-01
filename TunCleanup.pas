unit TunCleanup;

interface

// Removes Wintun adapters that are no longer started (left behind by a
// crashed or forcibly terminated client). Active adapters are left alone.
function RemoveStaleWintunAdapters: integer;

implementation

uses
  Winapi.Windows,
  System.SysUtils;

type
  HDEVINFO = THandle;

  TSPDevInfoData = record
    cbSize: DWORD;
    ClassGuid: TGUID;
    DevInst: DWORD;
    Reserved: ULONG_PTR;
  end;
  PSPDevInfoData = ^TSPDevInfoData;

  TSPClassInstallHeader = record
    cbSize: DWORD;
    InstallFunction: DWORD;
  end;
  PSPClassInstallHeader = ^TSPClassInstallHeader;

  TSPRemoveDeviceParams = record
    ClassInstallHeader: TSPClassInstallHeader;
    Scope: DWORD;
    HwProfile: DWORD;
  end;
  PSPRemoveDeviceParams = ^TSPRemoveDeviceParams;

const
  DIGCF_ALLCLASSES = $00000004;
  DIF_REMOVE = $00000005;
  DI_REMOVEDEVICE_GLOBAL = $00000001;
  DN_STARTED = $00000008;
  CR_SUCCESS = 0;

function SetupDiGetClassDevsW(ClassGuid: PGUID; Enumerator: PWideChar; hwndParent: HWND; Flags: DWORD): HDEVINFO;
  stdcall; external 'setupapi.dll';
function SetupDiEnumDeviceInfo(DeviceInfoSet: HDEVINFO; MemberIndex: DWORD;
  var DeviceInfoData: TSPDevInfoData): BOOL; stdcall; external 'setupapi.dll';
function SetupDiDestroyDeviceInfoList(DeviceInfoSet: HDEVINFO): BOOL; stdcall; external 'setupapi.dll';
function SetupDiGetDeviceInstanceIdW(DeviceInfoSet: HDEVINFO; const DeviceInfoData: TSPDevInfoData;
  DeviceInstanceId: PWideChar; DeviceInstanceIdSize: DWORD; RequiredSize: PDWORD): BOOL;
  stdcall; external 'setupapi.dll';
function SetupDiSetClassInstallParamsW(DeviceInfoSet: HDEVINFO; const DeviceInfoData: TSPDevInfoData;
  ClassInstallParams: PSPClassInstallHeader; ClassInstallParamsSize: DWORD): BOOL;
  stdcall; external 'setupapi.dll';
function SetupDiCallClassInstaller(InstallFunction: DWORD; DeviceInfoSet: HDEVINFO;
  const DeviceInfoData: TSPDevInfoData): BOOL; stdcall; external 'setupapi.dll';
function CM_Get_DevNode_Status(out pulStatus, pulProblemNumber: ULONG; dnDevInst: DWORD;
  ulFlags: ULONG): ULONG; stdcall; external 'cfgmgr32.dll';

function GetDeviceInstanceId(DeviceInfoSet: HDEVINFO; const DeviceInfoData: TSPDevInfoData): string;
var
  buffer: array [0 .. 1023] of WideChar;
  requiredSize: DWORD;
begin
  result := '';
  requiredSize := 0;
  if SetupDiGetDeviceInstanceIdW(DeviceInfoSet, DeviceInfoData, @buffer[0], Length(buffer), @requiredSize) then
    result := string(PWideChar(@buffer[0]));
end;

function IsDeviceStarted(const DeviceInfoData: TSPDevInfoData): boolean;
var
  status, problemNumber: ULONG;
begin
  status := 0;
  problemNumber := 0;
  result := (CM_Get_DevNode_Status(status, problemNumber, DeviceInfoData.DevInst, 0) = CR_SUCCESS) and
    ((status and DN_STARTED) <> 0);
end;

function RemoveDevice(DeviceInfoSet: HDEVINFO; const DeviceInfoData: TSPDevInfoData): boolean;
var
  params: TSPRemoveDeviceParams;
begin
  ZeroMemory(@params, SizeOf(params));
  params.ClassInstallHeader.cbSize := SizeOf(TSPClassInstallHeader);
  params.ClassInstallHeader.InstallFunction := DIF_REMOVE;
  params.Scope := DI_REMOVEDEVICE_GLOBAL;

  result := SetupDiSetClassInstallParamsW(DeviceInfoSet, DeviceInfoData,
    PSPClassInstallHeader(@params), SizeOf(params)) and
    SetupDiCallClassInstaller(DIF_REMOVE, DeviceInfoSet, DeviceInfoData);
end;

function RemoveStaleWintunAdapters: integer;
var
  deviceInfoSet: HDEVINFO;
  deviceInfo: TSPDevInfoData;
  index: DWORD;
  instanceId: string;
  removedInPass, removed: boolean;
  pass: integer;
begin
  result := 0;

  for pass := 1 to 16 do
  begin
    deviceInfoSet := SetupDiGetClassDevsW(nil, nil, 0, DIGCF_ALLCLASSES);
    if deviceInfoSet = INVALID_HANDLE_VALUE then
      exit;

    removedInPass := false;
    try
      index := 0;
      while true do
      begin
        ZeroMemory(@deviceInfo, SizeOf(deviceInfo));
        deviceInfo.cbSize := SizeOf(deviceInfo);

        if not SetupDiEnumDeviceInfo(deviceInfoSet, index, deviceInfo) then
          break;
        inc(index);

        instanceId := GetDeviceInstanceId(deviceInfoSet, deviceInfo);
        if not SameText(Copy(instanceId, 1, 11), 'SWD\WINTUN\') then
          continue;

        if IsDeviceStarted(deviceInfo) then
          continue;

        if RemoveDevice(deviceInfoSet, deviceInfo) then
        begin
          inc(result);
          removedInPass := true;
          break;
        end;
      end;
    finally
      SetupDiDestroyDeviceInfoList(deviceInfoSet);
    end;

    removed := removedInPass;
    if not removed then
      break;
  end;
end;

end.
