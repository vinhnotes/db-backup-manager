// Messages returned by the Go server (internal/manager), which are written
// in Vietnamese: [vi, en, ja]. {0}, {1}… capture the dynamic parts.
// The table lives next to the Go code so notifications use the same
// translations. Keep longer messages before shorter ones that they contain.
import table from "../../../internal/manager/server_messages.json";

const serverMessages = table as [string, string, string][];

export default serverMessages;
