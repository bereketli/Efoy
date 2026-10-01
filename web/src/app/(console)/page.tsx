import { ApiStatus } from "@/components/api-status";
import { TestMap } from "@/components/test-map";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export default function OverviewPage() {
  return (
    <div className="flex flex-col gap-6 p-6">
      <h1 className="text-2xl font-semibold">Overview</h1>
      <div className="grid gap-6 lg:grid-cols-[320px_1fr]">
        <ApiStatus />
        <Card>
          <CardHeader>
            <CardTitle>Addis Ababa</CardTitle>
            <CardDescription>Live vehicles will appear here once tracking is connected.</CardDescription>
          </CardHeader>
          <CardContent>
            <TestMap />
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
