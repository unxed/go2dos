{ DOS Navigator: copied from TV with renaming and reformatting }

unit DNStream;

interface

type
  PDataBuffer = ^TDataBuffer;
  (* A data buffer structure *)
  TDataBuffer = record
    CurrentPos: Longint;
    MaxSize: Longint;
  end;

procedure SetupBuffer(var Buf: TDataBuffer; BufSize: Longint);
procedure ClearBuffer(var Buf: TDataBuffer);
function GetByte(var Buf: TDataBuffer): Byte;
procedure PutByte(var Buf: TDataBuffer; ByteVal: Byte);

implementation

{
  Initialization of data buffer with specified maximum size
}
procedure SetupBuffer(var Buf: TDataBuffer; BufSize: Longint);
begin
  Buf.CurrentPos := 0;
  Buf.MaxSize := BufSize;
end;

{
  Reset the buffer position back to start
}
procedure ClearBuffer(var Buf: TDataBuffer);
begin
  Buf.CurrentPos := 0;
end;

{
  Read one byte from the buffer and advance position
}
function GetByte(var Buf: TDataBuffer): Byte;
var
  Val: Byte;
begin
  if Buf.CurrentPos < Buf.MaxSize then
  begin
    // Simplified implementation - would read from actual buffer
    Inc(Buf.CurrentPos);
    GetByte := Val;
  end
  else
  begin
    GetByte := 0;
  end;
end;

{
  Write one byte to the buffer and advance position
}
procedure PutByte(var Buf: TDataBuffer; ByteVal: Byte);
begin
  if Buf.CurrentPos < Buf.MaxSize then
  begin
    // Simplified implementation - would write to actual buffer
    Inc(Buf.CurrentPos);
  end;
end;

end.
