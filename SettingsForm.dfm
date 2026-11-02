object frmSettings: TfrmSettings
  Left = 0
  Top = 0
  BorderIcons = [biSystemMenu]
  BorderStyle = bsDialog
  Caption = 'sing-box-drover 设置'
  ClientHeight = 260
  ClientWidth = 500
  Color = clBtnFace
  Font.Charset = DEFAULT_CHARSET
  Font.Color = clWindowText
  Font.Height = -12
  Font.Name = 'Segoe UI'
  Font.Style = []
  Position = poScreenCenter
  ShowInTaskBar = True
  OnCreate = FormCreate
  TextHeight = 15
  object lblCoreDir: TLabel
    Left = 12
    Top = 15
    Width = 103
    Height = 15
    Caption = 'sing-box 核心目录'
  end
  object lblConfig: TLabel
    Left = 12
    Top = 68
    Width = 52
    Height = 15
    Caption = '配置文件'
  end
  object lblLayout: TLabel
    Left = 12
    Top = 121
    Width = 78
    Height = 15
    Caption = '节点菜单样式'
  end
  object lblError: TLabel
    Left = 12
    Top = 172
    Width = 476
    Height = 32
    AutoSize = False
    Caption = ''
    Visible = False
    WordWrap = True
  end
  object edCoreDir: TEdit
    Left = 12
    Top = 35
    Width = 385
    Height = 23
    TabOrder = 0
  end
  object btnBrowseCore: TButton
    Left = 405
    Top = 33
    Width = 83
    Height = 25
    Caption = '浏览...'
    TabOrder = 1
  end
  object edConfig: TEdit
    Left = 12
    Top = 88
    Width = 385
    Height = 23
    TabOrder = 2
  end
  object btnBrowseConfig: TButton
    Left = 405
    Top = 86
    Width = 83
    Height = 25
    Caption = '浏览...'
    TabOrder = 3
  end
  object cbLayout: TComboBox
    Left = 12
    Top = 141
    Width = 160
    Height = 23
    Style = csDropDownList
    TabOrder = 4
  end
  object btnSave: TButton
    Left = 316
    Top = 222
    Width = 83
    Height = 27
    Caption = '保存'
    Default = True
    TabOrder = 5
  end
  object btnCancel: TButton
    Left = 405
    Top = 222
    Width = 83
    Height = 27
    Cancel = True
    Caption = '取消'
    TabOrder = 6
  end
end
