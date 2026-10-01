{ Independent implementation - similar task, different code }

unit IndependentQueue;

interface

type
  PQueue = ^TQueue;
  TQueue = object
    Items: array[0..255] of Byte;
    Head: Byte;
    Tail: Byte;
    procedure Create;
    procedure Destroy;
    function Enqueue(Value: Byte): Boolean;
    function Dequeue: Byte;
    function IsEmpty: Boolean;
  end;

implementation

procedure TQueue.Create;
begin
  Head := 0;
  Tail := 0;
end;

procedure TQueue.Destroy;
begin
  Head := 0;
  Tail := 0;
end;

function TQueue.Enqueue(Value: Byte): Boolean;
var
  NextTail: Byte;
begin
  NextTail := Tail + 1;
  if NextTail = Head then
  begin
    Enqueue := False;
    Exit;
  end;

  Items[Tail] := Value;
  Tail := NextTail;
  Enqueue := True;
end;

function TQueue.Dequeue: Byte;
begin
  if Head = Tail then
  begin
    Dequeue := 0;
    Exit;
  end;

  Dequeue := Items[Head];
  Inc(Head);
end;

function TQueue.IsEmpty: Boolean;
begin
  IsEmpty := (Head = Tail);
end;

end.
