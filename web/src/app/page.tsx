import { Hero } from "@/components/landing/hero";
import { HowItWorks } from "@/components/landing/how-it-works";
import { Standards } from "@/components/landing/standards";
import { EntityTypes } from "@/components/landing/entity-types";
import { GetStarted } from "@/components/landing/get-started";

export default function Home() {
  return (
    <>
      <Hero />
      <HowItWorks />
      <Standards />
      <EntityTypes />
      <GetStarted />
    </>
  );
}
