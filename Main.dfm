object frmMain: TfrmMain
  Left = 0
  Top = 0
  Caption = 'sing-box-drover'
  ClientHeight = 278
  ClientWidth = 412
  Color = clBtnFace
  Font.Charset = DEFAULT_CHARSET
  Font.Color = clWindowText
  Font.Height = -12
  Font.Name = 'Segoe UI'
  Font.Style = []
  OnCloseQuery = FormCloseQuery
  OnCreate = FormCreate
  TextHeight = 15
  object PopupMenu: TPopupMenu
    Left = 248
    Top = 40
    object miExtras: TMenuItem
      Caption = '更多'
      object miRestart: TMenuItem
        Caption = '重启'
        OnClick = miRestartClick
      end
      object miAutostart: TMenuItem
        Caption = '开机自启'
        Enabled = False
        OnClick = miAutostartClick
      end
      object miHomepage: TMenuItem
        Caption = '项目主页'
        OnClick = miHomepageClick
      end
    end
    object miNodes: TMenuItem
      Caption = '节点'
      Visible = False
    end
    object miTun: TMenuItem
      Caption = 'TUN 模式'
      Visible = False
      OnClick = miTunClick
    end
    object miSystemProxy: TMenuItem
      Caption = '系统代理'
      OnClick = miSystemProxyClick
    end
    object miDashboard: TMenuItem
      Caption = '面板'
      Visible = False
      OnClick = miDashboardClick
    end
    object miQuit: TMenuItem
      Caption = '退出'
      OnClick = miQuitClick
    end
  end
end
