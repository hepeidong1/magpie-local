package sessions

// Agent trace readers are separate from the usage/listing cache: they retain
// just an incremental cursor and the active interaction, never session titles
// or completed conversations. They don't send data themselves.
