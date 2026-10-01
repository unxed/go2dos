{ Turbo Vision original procedures for control testing }

unit TVOriginal;

interface

type
  PStream = ^TStream;
  TStream = record
    Position: Longint;
    Size: Longint;
  end;

procedure InitStream(var S: TStream; ASize: Longint);
procedure ResetStream(var S: TStream);
function ReadByte(var S: TStream): Byte;
procedure WriteByte(var S: TStream; B: Byte);

implementation

procedure InitStream(var S: TStream; ASize: Longint);
begin
  S.Position := 0;
  S.Size := ASize;
end;

procedure ResetStream(var S: TStream);
begin
  S.Position := 0;
end;

function ReadByte(var S: TStream): Byte;
var
  Value: Byte;
begin
  if S.Position < S.Size then
  begin
    { Simplified: normally would read from buffer }
    Inc(S.Position);
    ReadByte := Value;
  end
  else
    ReadByte := 0;
end;

procedure WriteByte(var S: TStream; B: Byte);
begin
  if S.Position < S.Size then
  begin
    { Simplified: normally would write to buffer }
    Inc(S.Position);
  end;
end;

end.
